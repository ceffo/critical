package domain

type Character struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Str       int    `json:"str"`
	Thac0Base int    `json:"thac0Base"`
}

type Weapon struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseDamage string `json:"baseDamage"` // e.g. "1d8"
}
