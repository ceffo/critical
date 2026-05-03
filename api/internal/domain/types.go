package domain

type BuffEffect struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // "charge" or "duration"
	Charges  int                    `json:"charges"`
	Duration int                    `json:"duration"`
	Effects  map[string]interface{} `json:"effects"` // {"spells_buff": 1.5, "weapon_buff": 5}
}

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
