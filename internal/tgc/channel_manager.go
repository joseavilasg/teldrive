package tgc

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	storage "github.com/gotd/contrib/storage"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
	"github.com/tgdrive/teldrive/internal/auth"
	"github.com/tgdrive/teldrive/internal/cache"
	"github.com/tgdrive/teldrive/internal/config"
	"github.com/tgdrive/teldrive/internal/logging"
	"github.com/tgdrive/teldrive/internal/tgstorage"
	"github.com/tgdrive/teldrive/pkg/models"
	"github.com/tgdrive/teldrive/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoDefaultChannel = fmt.Errorf("no default channel found")
)

const addBotOperationTimeout = 1 * time.Minute

type ChannelManager struct {
	db    *gorm.DB
	cache cache.Cacher
	cnf   *config.TGConfig
}

func NewChannelManager(db *gorm.DB, cache cache.Cacher, cnf *config.TGConfig) *ChannelManager {
	return &ChannelManager{
		db:    db,
		cache: cache,
		cnf:   cnf,
	}
}

func (cm *ChannelManager) GetChannel(ctx context.Context, userID int64) (int64, error) {
	return cm.CurrentChannel(ctx, userID)
}

func (cm *ChannelManager) ChannelLimitReached(channelID int64) bool {
	var totalParts int64
	err := cm.db.Model(&models.File{}).
		Where("channel_id = ?", channelID).
		Select("COALESCE(SUM(CASE WHEN jsonb_typeof(parts) = 'array' THEN jsonb_array_length(parts) ELSE 0 END), 0) as total_parts").
		Scan(&totalParts).Error
	if err != nil {
		return false
	}
	return totalParts >= int64(cm.cnf.ChannelLimit)
}

func (cm *ChannelManager) CurrentChannel(ctx context.Context, userID int64) (int64, error) {
	return cache.Fetch(ctx, cm.cache, cache.KeyUserChannel(userID), 0, func() (int64, error) {
		var channelIds []int64
		if err := cm.db.Model(&models.Channel{}).Where("user_id = ?", userID).Where("selected = ?", true).
			Pluck("channel_id", &channelIds).Error; err != nil {
			return 0, err
		}
		if len(channelIds) == 0 {
			return 0, ErrNoDefaultChannel
		}
		return channelIds[0], nil
	})
}

func (cm *ChannelManager) BotTokens(ctx context.Context, userID int64) ([]string, error) {
	return cache.Fetch(ctx, cm.cache, cache.KeyUserBots(userID), 0, func() ([]string, error) {
		var bots []string
		if err := cm.db.Model(&models.Bot{}).Where("user_id = ?", userID).Pluck("token", &bots).Error; err != nil {
			return nil, err
		}
		return bots, nil
	})

}

func (cm *ChannelManager) CreateNewChannel(ctx context.Context, newChannelName string, userID int64, setDefault bool) (int64, error) {
	// Acquire distributed lock to prevent race conditions in multi-instance setups
	lockID := cm.generateLockID(userID, "channel_rollover")

	// Try to acquire lock (10 second timeout)
	lockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := cm.acquireAdvisoryLock(lockCtx, lockID); err != nil {
		return 0, fmt.Errorf("failed to acquire channel creation lock: %w", err)
	}
	defer cm.releaseAdvisoryLock(context.Background(), lockID)

	// Double-check: Was a channel created recently by another instance?
	recentChannel, err := cm.getChannelCreatedAfter(ctx, userID, time.Now().Add(-10*time.Second))
	if err != nil {
		return 0, err
	}
	if recentChannel != nil {
		// Another instance already created channel - use it!
		return recentChannel.ChannelId, nil
	}

	if newChannelName == "" {
		newChannelName = fmt.Sprintf("storage_%d", time.Now().Unix())
	}

	jwtUser := auth.GetJWTUser(ctx)
	if jwtUser == nil {
		return 0, fmt.Errorf("no JWT user found in context")
	}

	peerStorage := tgstorage.NewPeerStorage(cm.db, cache.KeyPeer(userID))
	middlewares := NewMiddleware(cm.cnf, WithFloodWait(), WithRetry(5), WithRateLimit())
	client, err := AuthClient(ctx, cm.cnf, jwtUser.TgSession, middlewares...)
	if err != nil {
		return 0, fmt.Errorf("failed to create Telegram client: %w", err)
	}

	var newChannelID int64
	var newChannel *tg.Channel

	err = client.Run(ctx, func(ctx context.Context) error {
		res, err := client.API().ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{
			Title:     newChannelName,
			Broadcast: true,
		})
		if err != nil {
			return err
		}

		updates := res.(*tg.Updates)
		var found bool
		for _, chat := range updates.Chats {
			if ch, ok := chat.(*tg.Channel); ok {
				newChannel = ch
				newChannelID = ch.ID
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("failed to extract channel from creation response")
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to create Telegram channel: %w", err)
	}

	peer := storage.Peer{}
	peer.FromChat(newChannel)
	peerStorage.Add(ctx, peer)
	botTokens, err := cm.BotTokens(ctx, userID)
	if err != nil {
		return 0, err
	}
	if len(botTokens) > 0 {
		err = cm.AddBotsToChannel(ctx, auth.GetJWTUser(ctx).TgSession, userID, newChannelID, botTokens, false)
		if err != nil {
			return 0, err
		}
	} else {
		return 0, fmt.Errorf("add bot tokens before continuing")
	}

	newChannelRecord := models.Channel{
		ChannelName: newChannelName,
		ChannelId:   newChannelID,
		UserId:      userID,
		Selected:    setDefault,
	}

	if setDefault {
		err = cm.db.Transaction(func(tx *gorm.DB) error {
			err := tx.Model(&models.Channel{}).Where("user_id = ?", userID).
				Update("selected", false).Error
			if err != nil {
				return err
			}
			return tx.Create(&newChannelRecord).Error
		})

		if err != nil {
			return 0, fmt.Errorf("failed to update channel database: %w", err)
		}
		cm.cache.Delete(ctx, cache.KeyUserChannel(userID))
	} else {
		if err := cm.db.Create(&newChannelRecord).Error; err != nil {
			return 0, fmt.Errorf("failed to create channel record: %w", err)
		}
	}

	return newChannelID, nil
}

// generateLockID creates a unique lock ID from user ID and operation
func (cm *ChannelManager) generateLockID(userID int64, operation string) int64 {
	// Use hash to generate unique int64 from userID + operation
	// PostgreSQL advisory locks use int64
	return userID*1000000 + int64(len(operation)) // Simple but effective
}

// acquireAdvisoryLock attempts to acquire PostgreSQL advisory lock
func (cm *ChannelManager) acquireAdvisoryLock(ctx context.Context, lockID int64) error {
	var acquired bool
	err := cm.db.WithContext(ctx).Raw(
		"SELECT pg_try_advisory_lock(?)", lockID,
	).Scan(&acquired).Error
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("lock already held by another instance")
	}
	return nil
}

// releaseAdvisoryLock releases PostgreSQL advisory lock
func (cm *ChannelManager) releaseAdvisoryLock(ctx context.Context, lockID int64) error {
	return cm.db.WithContext(ctx).Exec(
		"SELECT pg_advisory_unlock(?)", lockID,
	).Error
}

// getChannelCreatedAfter checks if a channel was created for user after given time
func (cm *ChannelManager) getChannelCreatedAfter(ctx context.Context, userID int64, after time.Time) (*models.Channel, error) {
	var channel models.Channel
	err := cm.db.WithContext(ctx).
		Where("user_id = ? AND created_at > ?", userID, after).
		Order("created_at DESC").
		First(&channel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No channel found
		}
		return nil, err
	}
	return &channel, nil
}

func (cm *ChannelManager) AddBotsToChannel(ctx context.Context, userSession string, userId int64, channelId int64, botsTokens []string, save bool) error {
	logger := logging.Component("TG").With(
		zap.Int64("user_id", userId),
		zap.Int64("channel_id", channelId),
		zap.Int("bot_count", len(botsTokens)),
		zap.Bool("save", save),
	)
	logger.Info("channel.add_bots.started")

	middlewares := NewMiddleware(cm.cnf, WithFloodWait(), WithRateLimit())

	client, err := AuthClient(ctx, cm.cnf, userSession, middlewares...)
	if err != nil {
		logger.Error("channel.add_bots.client_failed", zap.Error(err))
		return err
	}

	err = RunWithAuth(ctx, client, "", func(ctx context.Context) error {

		channel, err := GetChannelById(ctx, client.API(), channelId)

		if err != nil {
			logger.Error("channel.add_bots.channel_lookup_failed", zap.Error(err))
			return err
		}
		logger.Info("channel.add_bots.channel_resolved")

		errChan := make(chan error, len(botsTokens))

		infoChan := make(chan *types.BotInfo, len(botsTokens))

		g, ctx := errgroup.WithContext(ctx)

		g.SetLimit(4)

		for _, token := range botsTokens {
			token := token
			g.Go(func() error {
				botLogger := logger.With(zap.String("bot_token_id", botTokenID(token)))
				botLogger.Info("channel.add_bots.bot_started")
				var info *types.BotInfo

				backoffCfg := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3)

				err := backoff.RetryNotify(func() error {
					botCtx, cancel := context.WithTimeout(ctx, addBotOperationTimeout)
					defer cancel()
					var err error
					info, err = GetBotInfo(botCtx, cm.db, cm.cache, cm.cnf, token)
					if err != nil {
						botLogger.Warn("channel.add_bots.bot_info_failed", zap.Error(err))
						return err
					}
					botLogger.Info("channel.add_bots.bot_info_loaded", zap.Int64("bot_id", info.Id), zap.String("username", info.UserName))

					peerClass, err := peer.DefaultResolver(client.API()).ResolveDomain(botCtx, info.UserName)
					if err != nil {
						botLogger.Warn("channel.add_bots.resolve_failed", zap.String("username", info.UserName), zap.Error(err))
						return err
					}

					var ok bool
					botPeer, ok := peerClass.(*tg.InputPeerUser)
					if !ok {
						botLogger.Warn("channel.add_bots.invalid_peer", zap.String("username", info.UserName))
						return fmt.Errorf("invalid peer type for bot %s", info.UserName)
					}
					info.AccessHash = botPeer.AccessHash
					botLogger.Info("channel.add_bots.promote_started", zap.Int64("bot_id", info.Id))
					payload := &tg.ChannelsEditAdminRequest{
						Channel: channel,
						UserID:  tg.InputUserClass(&tg.InputUser{UserID: info.Id, AccessHash: info.AccessHash}),
						AdminRights: tg.ChatAdminRights{
							ChangeInfo:     true,
							PostMessages:   true,
							EditMessages:   true,
							DeleteMessages: true,
							BanUsers:       true,
							InviteUsers:    true,
							PinMessages:    true,
							ManageCall:     true,
							Other:          true,
							ManageTopics:   true,
						},
						Rank: "bot",
					}

					_, err = client.API().ChannelsEditAdmin(botCtx, payload)
					if err != nil {
						botLogger.Warn("channel.add_bots.promote_failed", zap.Error(err))
						return err
					}
					botLogger.Info("channel.add_bots.promote_completed", zap.Int64("bot_id", info.Id))
					return nil
				}, backoffCfg, nil)

				if err != nil {
					botLogger.Error("channel.add_bots.bot_failed", zap.Error(err))
					errChan <- err
					return nil
				}
				botLogger.Info("channel.add_bots.bot_completed", zap.Int64("bot_id", info.Id))
				infoChan <- info
				return nil
			})
		}

		done := make(chan struct{})
		go func() {
			g.Wait()
			close(infoChan)
			close(errChan)
			close(done)
		}()

		var botInfos []*types.BotInfo
		var botErrors []error

		for {
			select {
			case info, ok := <-infoChan:
				if ok {
					botInfos = append(botInfos, info)
				}
			case botErr, ok := <-errChan:
				if ok {
					botErrors = append(botErrors, botErr)
				}
			case <-done:
				logger.Info("channel.add_bots.finished",
					zap.Int("successful_bots", len(botInfos)),
					zap.Int("failed_bots", len(botErrors)),
				)
				if len(botErrors) > 2 {
					return fmt.Errorf("failed to process %d out of %d bots", len(botErrors), len(botsTokens))
				}
				if save && len(botInfos) > 0 {
					payload := []models.Bot{}
					for _, info := range botInfos {
						payload = append(payload, models.Bot{UserId: userId, Token: info.Token, BotId: info.Id})
					}
					if err := cm.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&payload).Error; err != nil {
						logger.Error("channel.add_bots.save_failed", zap.Error(err))
						return fmt.Errorf("failed to save bots: %w", err)
					}
					cm.cache.Delete(ctx, cache.KeyUserBots(userId))
					logger.Info("channel.add_bots.cache_invalidated")
				}
				return nil
			case <-ctx.Done():
				logger.Warn("channel.add_bots.context_done", zap.Error(ctx.Err()))
				return ctx.Err()
			}
		}
	})

	if err != nil {
		logger.Error("channel.add_bots.failed", zap.Error(err))
		return err
	}
	logger.Info("channel.add_bots.completed")
	return err
}
