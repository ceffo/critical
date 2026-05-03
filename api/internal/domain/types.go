package domain

type Character struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Str       int    `json:"str"`
	Dex       int    `json:"dex"`
	Con       int    `json:"con"`
	Int       int    `json:"int"`
	Wis       int    `json:"wis"`
	Cha       int    `json:"cha"`
	HPMax     int    `json:"hpMax"`
	Thac0Base int    `json:"thac0Base"`
	Level     int    `json:"level"`
	XPTotal   int    `json:"xpTotal"`

	// Saving Throws (target number, lower is better)
	SaveParaPoison   int `json:"saveParaPoison"`
	SaveRodStaffWand int `json:"saveRodStaffWand"`
	SavePetriPoly    int `json:"savePetriPoly"`
	SaveBreath       int `json:"saveBreath"`
	SaveSpell        int `json:"saveSpell"`
}

type Weapon struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseDamage string `json:"baseDamage"` // e.g. "1d8"
}
