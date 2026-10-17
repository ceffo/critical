// How the journal marks an entry: its kind's icon and the tone of its value. Shared by the journal
// and its collapsed bar.
import {
  BookOpen,
  Clock,
  CloudFog,
  DiceFive,
  MusicNotes,
  Flask,
  Heart,
  Hourglass,
  Lightning,
  MagicWand,
  Moon,
  Package,
  PencilSimple,
  Shield,
  Sparkle,
  Sword,
  TrendUp,
  type Icon,
} from "@phosphor-icons/react";
import type { RowValue } from "@/lib/filCard";
import type { LogKind } from "@/types/log";

export const KIND_ICONS: Record<LogKind, Icon> = {
  hp_change: Heart,
  cast: MagicWand,
  exposure: CloudFog,
  buff_change: Sparkle,
  round_advance: Hourglass,
  round_reset: Hourglass,
  round_set: Hourglass,
  time_passed: Clock,
  initiative: Lightning,
  attack: Sword,
  item_use: Flask,
  item_restock: Package,
  item_add: Package,
  item_edit: Package,
  item_remove: Package,
  weapon_add: Sword,
  weapon_edit: Sword,
  weapon_remove: Sword,
  xp_add: TrendUp,
  xp_edit: TrendUp,
  xp_remove: TrendUp,
  spell_prep: BookOpen,
  long_rest: Moon,
  save: Shield,
  skill_check: MusicNotes,
  dice: DiceFive,
  correction: PencilSimple,
};

export const VALUE_TONES: Record<RowValue["tone"], string> = {
  damage: "text-danger",
  heal: "text-success",
  pass: "text-success",
  fail: "text-danger",
  ink: "text-foreground",
  crit: "text-highlight",
  muted: "text-muted-foreground",
};
