/// <reference types="node" />
import { readFileSync } from "fs";
import { join } from "path";
import { describe, it, expect } from "vitest";

// The floating navbar docks to the top edge once the page scrolls, so scrolling
// content cannot show through the strip above it. Two halves have to agree and
// neither can see the other: 02_navbar.css styles `.refract-nav-docked`,
// refract.js puts it on <body>.
//
// Read with fs on purpose: vitest replaces every `.css?raw` import with an
// empty string, which would let these pass on nothing.

const navbarCss = readFileSync(
  join(__dirname, "../theme/css/02_navbar.css"),
  "utf8"
);
const refractJs = readFileSync(
  join(__dirname, "../../public/theme/refract.js"),
  "utf8"
);

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
    expect(navbarCss.length).toBeGreaterThan(1000);
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

  it("the docked bar gives back exactly the gap it gives up", () => {
    // WHY: docking moves the bar's top and sides out to the viewport edge. Its
    // buttons and bottom edge stay where they were only because the same
    // distance comes back as padding, on top of the padding the bar already
    // has. Change the float distance or the bar's own padding in one place and
    // not the other and every button jumps when the bar docks, and everything
    // offset from the bar's bottom edge goes with it.
    const floating = ruleBody(navbarCss, FLOATING);
    const docked = ruleBody(navbarCss, DOCKED);
    expect(floating).toMatch(/--nav-edge:\s*\d+px/);
    for (const side of ["top", "left", "right"]) {
      expect(floating).toMatch(
        new RegExp(`\\b${side}:\\s*var\\(--nav-edge\\)`)
      );
    }
    const own = /\bpadding:\s*([\d.]+rem)\s+([\d.]+rem)/.exec(floating);
    expect(own, "the floating bar's padding is no longer `Yrem Xrem`").not.toBe(
      null
    );
    const [, y, x] = own!;
    expect(docked).toContain(`padding-top: calc(${y} + var(--nav-edge))`);
    expect(docked).toContain(`padding-left: calc(${x} + var(--nav-edge))`);
    expect(docked).toContain(`padding-right: calc(${x} + var(--nav-edge))`);
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
