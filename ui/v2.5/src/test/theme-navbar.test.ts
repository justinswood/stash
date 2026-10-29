/// <reference types="node" />
import { existsSync, readFileSync } from "fs";
import { join } from "path";
import { describe, it, expect } from "vitest";

// The segmented-track navbar (theme/css/02_navbar.css, components/MainNavbar.tsx,
// public/theme/refract.js). The halves are written in different languages and
// none of them can see the others, so a rename or a changed number on one side
// leaves the bar or the page under it quietly wrong.
//
// Files are read with fs on purpose: vitest replaces every `.css?raw` import
// with an empty string, which would let these pass on nothing.

const srcDir = join(__dirname, "..");
const read = (rel: string) => readFileSync(join(srcDir, rel), "utf8");

const navbarCss = read("theme/css/02_navbar.css");
const tokensCss = read("theme/css/01_tokens.css");
const mainNavbar = read("components/MainNavbar.tsx");
const navIcons = read("components/NavIcons.tsx");
const refractJs = read("../public/theme/refract.js");
const indexHtml = read("../index.html");

const FLOATING = "body.stash-liquid-glass nav.top-nav.navbar";
const DOCKED = "body.stash-liquid-glass.refract-nav-docked nav.top-nav.navbar";

// The declarations of the first rule whose selector is exactly `selector`.
function ruleBody(css: string, selector: string): string {
  const start = css.indexOf(`${selector} {`);
  expect(start, `no rule for ${selector}`).toBeGreaterThanOrEqual(0);
  return css.slice(start, css.indexOf("}", start));
}

describe("navbar docks to the top edge on scroll", () => {
  it("reads the files it guards", () => {
    expect(navbarCss.length).toBeGreaterThan(5000);
    expect(refractJs.length).toBeGreaterThan(10000);
  });

  it("the CSS rule and refract.js agree on the class", () => {
    // renaming only one side would leave the bar floating with the gap back,
    // and nothing would report it
    expect(ruleBody(navbarCss, DOCKED)).toContain("top: 0");
    // boolean on purpose: a failing toContain would print all of refract.js
    expect(
      refractJs.includes('classList.toggle("refract-nav-docked"'),
      "refract.js no longer toggles .refract-nav-docked on body"
    ).toBe(true);
  });

  it("docked is the design's full-width variant, and only moves the bar", () => {
    // WHY: the design says that when the bar sits full-width it drops the
    // radius and the side borders and keeps only the bottom one. It must not
    // change the bar's size: the page under it (sticky toolbars, the scene
    // page's height) is laid out against the floating bar's bottom edge
    // (--nav-bottom), so docking just moves the bar up by --nav-edge.
    const floating = ruleBody(navbarCss, FLOATING);
    const docked = ruleBody(navbarCss, DOCKED);
    for (const side of ["top", "left", "right"]) {
      expect(floating).toMatch(
        new RegExp(`\\b${side}:\\s*var\\(--nav-edge\\)`)
      );
      expect(docked).toMatch(new RegExp(`\\b${side}:\\s*0\\b`));
    }
    expect(docked).toMatch(/border-radius:\s*0\b/);
    for (const side of ["top", "left", "right"]) {
      expect(docked).toMatch(
        new RegExp(`border-${side}-color:\\s*transparent`)
      );
    }
    expect(docked).not.toMatch(/\b(?:min-|max-)?height\s*:/);
    expect(docked).not.toMatch(/\bpadding[a-z-]*\s*:/);
  });

  it("the scroll handler toggles with force, never add/remove", () => {
    // WHY: it runs on every scroll event. classList.add/remove rewrite the
    // class attribute even when nothing changes, which wakes every
    // MutationObserver on body each time (refract.js has several).
    // toggle(name, force) is a no-op when the state already matches.
    expect(
      /classList\.(?:add|remove)\(\s*["']refract-nav-docked["']/.test(
        refractJs
      ),
      "refract.js uses classList.add/remove on .refract-nav-docked"
    ).toBe(false);
  });
});

describe("navbar size and the page under it", () => {
  it("--nav-bottom is the bar's edge plus its height", () => {
    // WHY: it is the one number everything below the bar is laid out from.
    expect(tokensCss).toMatch(/--nav-edge:\s*12px/);
    expect(tokensCss).toMatch(/--nav-height:\s*72px/);
    expect(tokensCss).toMatch(
      /--nav-bottom:\s*calc\(var\(--nav-edge\)\s*\+\s*var\(--nav-height\)\)/
    );
  });

  it.each([
    ["theme/css/04_filters.css", /top:\s*calc\(var\(--nav-bottom\)/],
    ["theme/css/13_plugins.css", /top:\s*calc\(var\(--nav-bottom\)/],
    [
      "theme/css/07_scene_details.css",
      /height:\s*calc\(100vh - var\(--nav-bottom\)/,
    ],
    ["theme/css/12_mobile.css", /top:\s*calc\(var\(--nav-bottom\)/],
    ["components/Performers/styles.scss", /var\(--nav-bottom/],
  ])("%s is offset from --nav-bottom", (file, pattern) => {
    // WHY: these used to hard-code the old, shorter bar's height. A literal
    // here puts the sticky toolbar, the performer sidebar, the scene page's
    // bottom edge or the mobile drawer under the bar, or leaves a gap, and
    // only on the pages that have them.
    expect(pattern.test(read(file)), `${file} has no --nav-bottom offset`).toBe(
      true
    );
  });

  it("the stock SCSS top offset follows the bar", () => {
    // $navbar-height is a compile-time constant, so it cannot read the CSS
    // variable. The bar is 84px to its bottom edge (12 + 72) and the stock
    // value that was tuned for a 63px edge was 48.75px: 84 - 14.25.
    const index = read("index.scss");
    expect(index).toMatch(/\$navbar-height:\s*69\.75px/);
    expect(index).toMatch(/\$mobile-navbar-height:\s*48\.75px/);
  });
});

describe("navbar markup hooks", () => {
  it.each([
    "nav-recents",
    "nav-stats",
    "nav-account",
    "nav-avatar",
    "navbar-search-kbd",
  ])("%s is used by both the CSS and MainNavbar.tsx", (cls) => {
    // WHY: the order of the icon row, the avatar and the shortcut chip are
    // all keyed on these class names; renaming one side drops the item into
    // the wrong place, or unstyled, without an error.
    expect(navbarCss).toMatch(new RegExp(`\\.${cls}(?![\\w-])`));
    expect(mainNavbar).toContain(cls);
  });

  it("refract.js leaves the navbar icons that React drew alone", () => {
    // WHY: refractApplyNavIcons swaps the svg inside any nav link whose href
    // is one of its routes (/stats, /scenes...) for the mobile drawer's
    // glyph. NavIcons.tsx marks its svgs data-nav-icon so that pass skips
    // them; without the guard the design's icons are replaced after render.
    expect(navIcons).toContain("data-nav-icon");
    expect(refractJs.includes('hasAttribute("data-nav-icon")')).toBe(true);
  });

  it("the navbar's search box is not mistaken for a list filter bar", () => {
    // WHY: initFilterBar tags the nearest div with >= 4 buttons around the
    // first input with a "Search" placeholder. The navbar search now has one,
    // and its button cluster would be tagged and turned into a wrapping flex
    // toolbar (see the categories-toolbar note in refract.js).
    expect(mainNavbar).toMatch(/placeholder=\{intl\.formatMessage/);
    expect(refractJs.includes(".categories-toolbar, nav.top-nav'")).toBe(true);
  });

  it("the search input has no corner radius, so its caret is not clipped", () => {
    // WHY: the input has no padding, so its caret sits on the input's own
    // edge. The theme rounds every .form-control (12px: a full pill at 19px
    // tall) and Chrome clips an input's contents to that rounded box, which
    // left a ~9px sliver of the caret visible: it read as a hidden cursor.
    // Found by diffing blink frames, not by reading the CSS.
    const rule = ruleBody(
      navbarCss,
      "body.stash-liquid-glass nav.top-nav .navbar-performer-search input.form-control"
    );
    expect(
      /border-radius:\s*0\s*!important/.test(rule),
      "the navbar search input is rounded again: its caret will be clipped"
    ).toBe(true);
  });
});

describe("navbar typeface", () => {
  it("Inter is declared, bundled and preloaded", () => {
    // WHY: the design is Inter only, and the tab widths depend on it. If a
    // file goes missing the font quietly falls back to the theme font, and the
    // bar no longer matches the design.
    expect(tokensCss).toMatch(/font-family:\s*"Inter"/);
    for (const file of [
      "Inter-Variable.latin.woff2",
      "Inter-Variable.latin-ext.woff2",
      "Inter-OFL.txt",
    ]) {
      expect(
        existsSync(join(srcDir, "../public/theme/fonts", file)),
        `${file} is missing`
      ).toBe(true);
    }
    expect(indexHtml).toContain("/theme/fonts/Inter-Variable.latin.woff2");
  });
});
