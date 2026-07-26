import { describe, it, expect } from "vitest";
import {
  computeLayout,
  HEADER_H,
  CELL_MIN,
  FOOTER_H_TOUCH_PORTRAIT,
  FOOTER_H_TOUCH_LANDSCAPE,
} from "./layout";

describe("computeLayout", () => {
  it("produces square cells (cols*cell <= availW, rows*cell <= availH)", () => {
    // 800x600 is landscape
    const L = computeLayout(800, 600, {
      cols: 30,
      rows: 22,
      enableDPad: true,
      hasDebugButton: false,
    });
    expect(L.cell).toBeGreaterThan(0);
    expect(L.gridW).toBe(L.cols * L.cell);
    expect(L.gridH).toBe(L.rows * L.cell);
    expect(L.gridW).toBeLessThanOrEqual(800);
    expect(L.gridH).toBeLessThanOrEqual(600 - HEADER_H - FOOTER_H_TOUCH_LANDSCAPE);
  });

  it("uses smaller footer in landscape for larger grid", () => {
    const L = computeLayout(800, 600, {
      cols: 30, rows: 22, enableDPad: true, hasDebugButton: false,
    });
    expect(L.orientation).toBe("landscape");
    expect(L.footerH).toBe(FOOTER_H_TOUCH_LANDSCAPE);
    expect(FOOTER_H_TOUCH_PORTRAIT).toBe(0);
    expect(FOOTER_H_TOUCH_LANDSCAPE).toBeGreaterThan(FOOTER_H_TOUCH_PORTRAIT);
  });

  it("clamps to CELL_MIN on absurdly small viewports", () => {
    const L = computeLayout(100, 100, {
      cols: 30,
      rows: 22,
      enableDPad: false,
      hasDebugButton: false,
    });
    expect(L.cell).toBeGreaterThanOrEqual(CELL_MIN);
  });

  it("reports portrait vs landscape correctly", () => {
    expect(
      computeLayout(400, 800, {
        cols: 30,
        rows: 22,
        enableDPad: true,
        hasDebugButton: false,
      }).orientation,
    ).toBe("portrait");
    expect(
      computeLayout(800, 400, {
        cols: 30,
        rows: 22,
        enableDPad: true,
        hasDebugButton: false,
      }).orientation,
    ).toBe("landscape");
  });

  it("centers grid horizontally (offsetX >= 0)", () => {
    const L = computeLayout(1200, 800, {
      cols: 30,
      rows: 22,
      enableDPad: true,
      hasDebugButton: false,
    });
    expect(L.offsetX).toBeGreaterThanOrEqual(0);
    expect(L.offsetX + L.gridW).toBeLessThanOrEqual(L.vw);
  });

  it("respects header reservation", () => {
    const L = computeLayout(1000, 500, {
      cols: 30,
      rows: 22,
      enableDPad: false,
      hasDebugButton: false,
    });
    expect(L.offsetY).toBeGreaterThanOrEqual(HEADER_H);
  });

  it("D-pad geometry falls within touch-friendly range", () => {
    const L = computeLayout(800, 600, {
      cols: 30,
      rows: 22,
      enableDPad: true,
      hasDebugButton: false,
    });
    expect(L.dpadStep).toBeGreaterThanOrEqual(44);
    expect(L.dpadStep).toBeLessThanOrEqual(64);
    expect(L.dpadFontSize).toBeGreaterThanOrEqual(22);
  });
});
