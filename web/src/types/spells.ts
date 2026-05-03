export interface SpellSlot {
  level: number;
  prepared_spell_name: string;
  used: boolean;
}

export interface Spell {
  name: string;
  class: string;
  level: number;
  school: string;
  sphere: string;
  casting_time: string;
  range: string;
  duration: string;
  area_of_effect: string;
  components: string;
  saving_throw: string;
  description: string;
}
