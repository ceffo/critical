/**
 * Drops late replies: each `open()` returns a check that stays true until the next `open()` or
 * `close()`, so a request answered after a newer one was sent (or after the screen moved on) is
 * ignored instead of overwriting the newer state.
 */
export interface ReplyGate {
  open: () => () => boolean;
  close: () => void;
}

export function replyGate(): ReplyGate {
  let at = 0;
  return {
    open: () => {
      const mine = ++at;
      return () => at === mine;
    },
    close: () => {
      at += 1;
    },
  };
}
