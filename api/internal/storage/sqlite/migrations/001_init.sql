CREATE TABLE IF NOT EXISTS characters (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    str         INTEGER NOT NULL,
    dex         INTEGER NOT NULL,
    con         INTEGER NOT NULL,
    int         INTEGER NOT NULL,
    wis         INTEGER NOT NULL,
    cha         INTEGER NOT NULL,
    hp_max      INTEGER NOT NULL,
    thac0_base  INTEGER NOT NULL,
    level       INTEGER NOT NULL DEFAULT 1,
    xp_total    INTEGER NOT NULL DEFAULT 0
);
