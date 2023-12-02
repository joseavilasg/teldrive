.PHONY:	pre-ui
pre-ui:
	cd ui/drive-ui && npm ci

.PHONY:	ui
ui:	
	cd ui/drive-ui && npm run build

.PHONY: sync-ui
sync-ui:
	git submodule update --init --recursive --remote
	

.PHONY: drive
drive:
	go build -trimpath -ldflags "-s -w -extldflags=-static"