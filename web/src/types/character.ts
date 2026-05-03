export interface Character {
  id: number;
  name: string;
  str: number;
  dex: number;
  con: number;
  int: number;
  wis: number;
  cha: number;
  hpMax: number;
  thac0Base: number;
  level: number;
  xpTotal: number;

  // Saving Throws
  saveParaPoison: number;
  saveRodStaffWand: number;
  savePetriPoly: number;
  saveBreath: number;
  saveSpell: number;
}
