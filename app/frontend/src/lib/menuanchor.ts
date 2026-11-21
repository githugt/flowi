// Where a menu opens from. A menu is placed by a point and which edge of the menu sits on it, and is kept inside the
// window by the menu itself once it knows how big it is.
// Before: each menu had its own CSS (`top: calc(100% + 4px); right: 0`) relative to a wrapper that had to be made
// `position: relative`, and a window pointer handler that closed it.

export interface MenuAt {
  x: number
  y: number
  /** The menu's right edge is on x, not its left. */
  end?: boolean
  /** The menu's bottom edge is on y, not its top: it opens upward. */
  above?: boolean
  /** Where the menu's bottom edge goes if it does not fit below y and there is more room above: just over the button. */
  flipTo?: number
}

export interface AnchorOptions {
  end?: boolean
  above?: boolean
  /** Space between the button and the menu. */
  gap?: number
  /** Open over the button instead when the menu does not fit below it (a select-like list near the window's bottom). */
  flip?: boolean
}

/** The point a menu opens at for a button: under it (or over it), by its left edge (or its right). */
export function anchorAt(rect: Pick<DOMRect, 'left' | 'right' | 'top' | 'bottom'>, o: AnchorOptions = {}): MenuAt {
  const gap = o.gap ?? 4
  return {
    x: o.end ? rect.right : rect.left,
    y: o.above ? rect.top - gap : rect.bottom + gap,
    end: o.end,
    above: o.above,
    ...(o.flip && !o.above ? { flipTo: rect.top - gap } : {}),
  }
}

/** The left and top of a menu of the given size opened at `at`, kept `margin` inside a window of the given size. */
export function menuPlace(at: MenuAt, size: { w: number; h: number }, room: { w: number; h: number }, margin = 8): { left: number; top: number } {
  const clamp = (n: number, room1: number, size1: number) => Math.max(margin, Math.min(n, room1 - size1 - margin))
  const below = room.h - margin - at.y
  const flip = !at.above && at.flipTo !== undefined && size.h > below && at.flipTo - margin > below
  return {
    left: clamp(at.end ? at.x - size.w : at.x, room.w, size.w),
    top: clamp(at.above ? at.y - size.h : flip ? at.flipTo! - size.h : at.y, room.h, size.h),
  }
}
