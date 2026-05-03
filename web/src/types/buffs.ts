export interface Buff {
  id: number;
  name: string;
  durationType: 'rounds' | 'charges';
  remaining: number;
}
