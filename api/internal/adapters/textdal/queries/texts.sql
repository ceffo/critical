-- GetText
SELECT lang, body FROM localized_texts WHERE text_id = ?
;

-- DeleteText
DELETE FROM localized_texts WHERE text_id = ?
;

-- InsertText
INSERT INTO localized_texts (text_id, lang, body) VALUES (?, ?, ?)
;
