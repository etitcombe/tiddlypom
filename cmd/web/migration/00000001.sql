CREATE TABLE tiddler (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	title      TEXT NOT NULL UNIQUE,
	rev        INTEGER NOT NULL,
	meta       TEXT NOT NULL,
	text       TEXT NOT NULL,
	is_system  INTEGER NOT NULL DEFAULT (0)
);

CREATE INDEX tiddler_title_idx ON tiddler (title);
CREATE INDEX tiddler_is_system_idx ON tiddler (is_system);
