/**
 * Pure, Phaser-free layout calculator for Snakey.
 *
 * Given a viewport size and a few flags, it returns a `Layout` object that
 * every scene and touch-control module reads to position / size elements.
 * This is the single source of truth for on-screen geometry.
 */

export const HEADER_H = 40;
export const FOOTER_H_TOUCH_PORTRAIT = 0;
export const FOOTER_H_TOUCH_LANDSCAPE = 100;
export const CELL_MIN = 8;
export const DPAD_EDGE_PAD = 90;

export type Orientation = "portrait" | "landscape";

export interface Layout {
  vw: number;
  vh: number;
  cols: number;
  rows: number;
  cell: number;
  offsetX: number;
  offsetY: number;
  gridW: number;
  gridH: number;
  headerH: number;
  footerH: number;
  orientation: Orientation;
  enableDPad: boolean;
  // HUD anchor points (canvas px)
  scoreX: number;
  scoreY: number;
  bestX: number;
  bestY: number;
  menuBtnX: number;
  menuBtnY: number;
  debugBtnX: number;
  debugBtnY: number;
  gameOverX: number;
  gameOverY: number;
  // D-pad geometry (canvas px)
  dpadCx: number;
  dpadCy: number;
  dpadStep: number;
  dpadFontSize: number;
}

export interface LayoutOpts {
  cols: number;
  rows: number;
  enableDPad: boolean;
  hasDebugButton: boolean;
}

export function computeLayout(
  vw: number,
  vh: number,
  opts: LayoutOpts,
): Layout {
  const orientation: Orientation = vh > vw ? "portrait" : "landscape";
  const footerH = opts.enableDPad
    ? orientation === "portrait"
      ? FOOTER_H_TOUCH_PORTRAIT
      : FOOTER_H_TOUCH_LANDSCAPE
    : 0;
  const availW = vw;
  const availH = Math.max(vh - HEADER_H - footerH, 1);

  const cell = Math.max(
    CELL_MIN,
    Math.floor(Math.min(availW / opts.cols, availH / opts.rows)),
  );
  const gridW = opts.cols * cell;
  const gridH = opts.rows * cell;
  const offsetX = Math.floor((vw - gridW) / 2);
  const offsetY = HEADER_H + 8;

  // D-pad scales modestly with cell, clamped to touch-friendly range
  const dpadStep = clampInt(cell * 1.6, 44, 64);
  const dpadFontSize = clampInt(cell * 1.1, 22, 40);

  return {
    vw,
    vh,
    cols: opts.cols,
    rows: opts.rows,
    cell,
    offsetX,
    offsetY,
    gridW,
    gridH,
    headerH: HEADER_H,
    footerH,
    orientation,
    enableDPad: opts.enableDPad,
    scoreX: 80,
    scoreY: 10,
    bestX: vw - (opts.hasDebugButton ? 200 : 16),
    bestY: 10,
    menuBtnX: 16,
    menuBtnY: 20,
    debugBtnX: vw - 16,
    debugBtnY: 20,
    gameOverX: vw / 2,
    gameOverY: vh / 2,
    dpadCx: vw - DPAD_EDGE_PAD,
    dpadCy: vh - DPAD_EDGE_PAD,
    dpadStep,
    dpadFontSize,
  };
}

function clampInt(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, Math.round(v)));
}
