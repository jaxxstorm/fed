ALTER TABLE login_attempts ADD COLUMN return_path text NOT NULL DEFAULT '/';
