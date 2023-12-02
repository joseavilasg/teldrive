-- +goose Up

-- +goose StatementBegin
CREATE PROCEDURE drive.update_size() LANGUAGE PLPGSQL AS $$
DECLARE
    rec RECORD;
    total_size BIGINT;
BEGIN
    FOR rec IN
        SELECT id
        FROM files
        WHERE type = 'folder'
        ORDER BY depth DESC
    LOOP
        total_size := (
            SELECT SUM(size) AS total_size
            FROM drive.files
            WHERE parent_id = rec.id
        );

        UPDATE drive.files
        SET size = total_size
        WHERE id = rec.id;
    END LOOP;
END;
$$;

CREATE PROCEDURE drive.delete_files(IN file_ids TEXT[], IN op TEXT DEFAULT 'bulk') LANGUAGE PLPGSQL AS $$
DECLARE
    rec RECORD;
BEGIN
    IF op = 'bulk' THEN
        FOR rec IN
            SELECT id, type
            FROM drive.files
            WHERE id = ANY (file_ids)
        LOOP
            IF rec.type = 'folder' THEN
                CALL drive.delete_files(ARRAY[rec.id], 'single');
            END IF;

            DELETE FROM drive.files
            WHERE id = rec.id;
        END LOOP;
    ELSE
        FOR rec IN
            SELECT id, type
            FROM drive.files
            WHERE parent_id = file_ids[1]
        LOOP
            IF rec.type = 'folder' THEN
                CALL drive.delete_files(ARRAY[rec.id], 'single');
            END IF;

            DELETE FROM drive.files
            WHERE id = rec.id;
        END LOOP;
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION drive.create_directories(
    IN tg_id BIGINT,
    IN long_path TEXT
) RETURNS SETOF drive.files AS $$
DECLARE
    path_parts TEXT[];
    current_directory_id TEXT;
    new_directory_id TEXT;
    directory_name TEXT;
    path_so_far TEXT;
    depth_dir INTEGER;
BEGIN
    path_parts := string_to_array(regexp_replace(long_path, '^/+', ''), '/');

    path_so_far := '';
    depth_dir := 0;

    SELECT id INTO current_directory_id
    FROM drive.files
    WHERE parent_id = 'root' AND user_id = tg_id;

    FOR directory_name IN SELECT unnest(path_parts) LOOP
        path_so_far := CONCAT(path_so_far, '/', directory_name);
        depth_dir := depth_dir + 1;

        SELECT id INTO new_directory_id
        FROM drive.files
        WHERE parent_id = current_directory_id
          AND "name" = directory_name
          AND "user_id" = tg_id;

        IF new_directory_id IS NULL THEN
            INSERT INTO drive.files ("name", "type", mime_type, parent_id, "user_id", starred, "depth", "path")
            VALUES (directory_name, 'folder', 'drive/folder', current_directory_id, tg_id, false, depth_dir, path_so_far)
            RETURNING id INTO new_directory_id;
        END IF;

        current_directory_id := new_directory_id;
    END LOOP;

    RETURN QUERY SELECT * FROM drive.files WHERE id = current_directory_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION drive.split_path(path TEXT, OUT parent TEXT, OUT base TEXT) AS $$
BEGIN
    IF path = '/' THEN
        parent := '/';
        base := NULL;
        RETURN;
    END IF;

    IF LEFT(path, 1) <> '/' THEN
        path := '/' || path;
    END IF;

    IF RIGHT(path, 1) = '/' THEN
        path := LEFT(path, LENGTH(path) - 1);
    END IF;

    parent := LEFT(path, LENGTH(path) - POSITION('/' IN REVERSE(path)));
    base := RIGHT(path, POSITION('/' IN REVERSE(path)) - 1);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION drive.update_folder(
  folder_id TEXT,
  new_name TEXT,
  new_path TEXT DEFAULT NULL
) RETURNS SETOF drive.files
LANGUAGE plpgsql
AS $$
DECLARE
  folder RECORD;
  path_items TEXT[];
BEGIN
  IF new_path IS NULL THEN
      SELECT
          *
      INTO
          folder
      FROM
          drive.files
      WHERE
          id = folder_id;
      
      path_items := STRING_TO_ARRAY(folder.path, '/');
      
      path_items[ARRAY_LENGTH(path_items, 1)] := new_name;
      
      new_path := ARRAY_TO_STRING(path_items, '/');
  END IF;
  
  UPDATE
      drive.files
  SET
      path = new_path,
      name = new_name
  WHERE
      id = folder_id;
  
  FOR folder IN
      SELECT
          *
      FROM
          drive.files
      WHERE
          type = 'folder'
          AND parent_id = folder_id
  LOOP
     PERFORM from drive.update_folder(
          folder.id,
          folder.name,
          CONCAT(new_path, '/', folder.name)
      );
  END LOOP;
 
  RETURN QUERY
  SELECT
      *
  FROM
      drive.files
  WHERE
      id = folder_id;
END;
$$;

CREATE OR REPLACE FUNCTION drive.move_directory(src TEXT, dest TEXT, u_id BIGINT) RETURNS VOID AS $$
DECLARE
    src_parent TEXT;
    src_base TEXT;
    dest_parent TEXT;
    dest_base TEXT;
    dest_id TEXT;
    src_id TEXT;
BEGIN
	
    IF NOT EXISTS (SELECT 1 FROM drive.files WHERE path = src AND user_id = u_id) THEN
        RAISE EXCEPTION 'source directory not found';
    END IF;
   
    IF EXISTS (SELECT 1 FROM drive.files WHERE path = dest AND user_id = u_id) THEN
        RAISE EXCEPTION 'destination directory exists';
    END IF;
   
    SELECT parent, base INTO src_parent, src_base FROM drive.split_path(src);
   
    SELECT parent, base INTO dest_parent, dest_base FROM drive.split_path(dest);
   
    IF src_parent != dest_parent THEN
      SELECT id INTO dest_id FROM drive.create_directories(u_id, dest);
      UPDATE drive.files SET parent_id = dest_id WHERE parent_id = (SELECT id FROM drive.files WHERE path = src) AND id != dest_id AND user_id = u_id;
      
      IF POSITION(CONCAT(src, '/') IN dest) = 0 THEN
         DELETE FROM drive.files WHERE path = src AND user_id = u_id;
      END IF;
     
    END IF;

    IF src_base != dest_base AND src_parent = dest_parent THEN
       SELECT id INTO src_id FROM drive.files WHERE path = src AND user_id = u_id;
       PERFORM from drive.update_folder(src_id, dest_base);
    END IF;

END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
DROP PROCEDURE IF EXISTS drive.update_size;
DROP PROCEDURE IF EXISTS drive.delete_files;
