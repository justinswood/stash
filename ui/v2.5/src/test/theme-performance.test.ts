/// <reference types="node" />
import { readFileSync } from "fs";
import { join } from "path";
import { describe, it, expect } from "vitest";
import { getSlickSliderSettings } from "src/core/recommendations";

// These guard two changes that cut the front page's main-thread blocking from
// ~5 s to ~1.5 s (dev server, headless Chrome): see the Performance entry in
// the fork modifications list. Both are easy to undo by accident.
//
// The files are read with fs on purpose: vitest replaces every `.css?raw`
// import with an empty string, which would make the CSS scan below pass
// vacuously.

const srcDir = join(__dirname, "..");
const read = (rel: string) => readFileSync(join(srcDir, rel), "utf8");

// Only the stylesheets theme/index.ts actually bundles. multiview-player.css is
// deliberately not one of them (it belongs to a separate page).
const bundled = [
  ...read("theme/index.ts").matchAll(/import\s+"\.\/css\/([^"]+\.css)"/g),
].map((m) => m[1]);
const css = new Map(bundled.map((n) => [n, read(`theme/css/${n}`)]));

function splitTopLevel(list: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let cur = "";
  for (const ch of list) {
    if (ch === "(") depth++;
    else if (ch === ")") depth--;
    if (ch === "," && depth === 0) {
      parts.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur.trim()) parts.push(cur.trim());
  return parts;
}

// Every selector in a stylesheet. Comments and string literals are removed first
// so prose in a comment, or a data: URI, cannot masquerade as a selector.
function selectorsOf(source: string): string[] {
  const clean = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'/g, '""');
  const out: string[] = [];
  const re = /([^{}]+)\{/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(clean))) {
    const prelude = m[1].split(";").pop()!.trim();
    if (!prelude || prelude.startsWith("@")) continue;
    out.push(...splitTopLevel(prelude));
  }
  return out;
}

// The leading compound selector: everything up to the first top-level combinator.
function firstCompound(selector: string): string {
  let depth = 0;
  let i = 0;
  for (; i < selector.length; i++) {
    const ch = selector[i];
    if (ch === "(") depth++;
    else if (ch === ")") depth--;
    else if (depth === 0 && /[\s>+~]/.test(ch)) break;
  }
  return selector.slice(0, i);
}

// html/body/:root with :has() in the same leading compound, e.g.
//   body.stash-liquid-glass:has(.Lightbox) .popover
function isRootedHas(selector: string): boolean {
  const compound = firstCompound(selector.trim());
  return /^(?:html|body|:root)/.test(compound) && compound.includes(":has(");
}

describe("theme CSS performance", () => {
  it("detects root-anchored :has() selectors (so the guard below can fail)", () => {
    expect(
      isRootedHas("body.stash-liquid-glass:has(.Lightbox) .popover.show")
    ).toBe(true);
    expect(isRootedHas("body:has(#mv-menu-panel) #mv-roulette-btn")).toBe(true);
    expect(isRootedHas("html.dark:has([data-x]) *")).toBe(true);
    expect(isRootedHas(":root:has(.a)")).toBe(true);
    // not rooted at the document: cheap, only re-evaluated for that subtree
    expect(isRootedHas(".scene-card:has(.stash-performer-link:hover)")).toBe(
      false
    );
    expect(isRootedHas("body.stash-liquid-glass .row:has(> .x)")).toBe(false);
    expect(isRootedHas("body.stash-liquid-glass.lightbox-open .card")).toBe(
      false
    );
  });

  it("splits selector lists and ignores comments and strings", () => {
    const sample = `
      /* body:has(.Lightbox) is mentioned in prose only */
      a, body:has(.x) b, :is(c, d) { background: url("data:image/svg+xml,{x}"); }
      @media (max-width: 1px) { body.k:has(.y) { color: red } }
    `;
    const rooted = selectorsOf(sample).filter(isRootedHas);
    expect(rooted).toEqual(["body:has(.x) b", "body.k:has(.y)"]);
  });

  it("really reads the stylesheets the theme bundles", () => {
    // Non-vacuous: an empty or missing read must fail here rather than let the
    // scan below pass on nothing.
    expect(bundled.length).toBeGreaterThan(10);
    for (const [name, source] of css) {
      expect(source.length, `${name} read as empty`).toBeGreaterThan(1000);
    }
    const total = [...css.values()].reduce(
      (n, source) => n + selectorsOf(source).length,
      0
    );
    expect(total).toBeGreaterThan(3000);
    expect(bundled).not.toContain("multiview-player.css");
  });

  it("has no html/body/:root-rooted :has() selector in the bundled CSS", () => {
    const offenders: string[] = [];
    for (const [name, source] of css) {
      for (const sel of selectorsOf(source)) {
        if (isRootedHas(sel)) offenders.push(`${name}: ${sel}`);
      }
    }
    // WHY: a :has() rooted at body or html makes the browser re-check the whole
    // document on every DOM change. 18 such rules cost the home page ~4.5 s of
    // style recalculation, because refract.js adds DOM constantly. Key the rule
    // off a class set by the component that creates the state instead, e.g.
    // `body.lightbox-open` (Lightbox.tsx) or `body.refract-float-pager`
    // (refract.js initFloatingPager).
    expect(offenders).toEqual([]);
  });

  it.each([
    ["lightbox-open", "hooks/Lightbox/Lightbox.tsx"],
    ["gallery-layout", "components/Galleries/GalleryDetails/Gallery.tsx"],
  ])(
    "the .%s selectors in the theme CSS have a component that sets the class",
    (cls, componentPath) => {
      // The CSS and the component agree on the name; renaming only one side
      // would leave the rules silently never matching.
      const allCss = [...css.values()].join("\n");
      const component = read(componentPath);
      // boundary-aware: `.lightbox-opened` must not satisfy `.lightbox-open`
      expect(allCss).toMatch(new RegExp(`\\.${cls}(?![\\w-])`));
      expect(component).toContain(`classList.add("${cls}")`);
      expect(component).toContain(`classList.remove("${cls}")`);
    }
  );
});

describe("front page sliders", () => {
  it.each([false, true])(
    "never loop, so slick renders no clones (isTouch=%s)",
    (isTouch) => {
      // WHY: infinite mode renders clones of the slides on both sides of the
      // real ones: 75 slides for a 25-item row, which tripled the front page's
      // DOM (23k nodes vs 8k) and made it the heaviest page by far.
      expect(getSlickSliderSettings(25, isTouch).infinite).toBe(false);
    }
  );
});
