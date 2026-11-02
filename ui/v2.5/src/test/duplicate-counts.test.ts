/// <reference types="node" />
import { readFileSync } from "fs";
import { join } from "path";
import { describe, it, expect } from "vitest";

// Two screens count "duplicates" and used to disagree: the Dashboard said 193
// and the duplicate checker said 84. They were never the same measure.
//
// - The Dashboard counted scenes whose phash repeats on any file. 189 of the
//   193 were ONE scene holding the same video twice, which the checker can
//   never list (its SQL needs the phash in two different scenes).
// - The checker's 84 was a count of SETS (253 scenes), which refract.js
//   relabelled "duplicates"; its "20 sets" and "97 GB" were page 1 of 5.
//
// These pin the pairing of every count with the list its link opens, and the
// unit on the set count. The halves are in different files and languages, so a
// change to one side leaves the screens quietly disagreeing again.
//
// Files are read with fs on purpose: vitest replaces every `.css?raw` import
// with an empty string, and a guard that reads nothing passes on nothing.

const srcDir = join(__dirname, "..");
const read = (rel: string) => readFileSync(join(srcDir, rel), "utf8");

const dashboard = read("components/Dashboard/Dashboard.tsx");
const checker = read(
  "components/SceneDuplicateChecker/SceneDuplicateChecker.tsx"
);
const refractJs = read("../public/theme/refract.js");

// The source of one Library Health row: from its label to the next row.
function healthRow(label: string): string {
  const start = dashboard.indexOf(`label: "${label}"`);
  expect(start, `no Library Health row labelled ${label}`).toBeGreaterThan(0);
  const next = dashboard.indexOf("{ label:", start + 1);
  return dashboard.slice(start, next > 0 ? next : undefined);
}

describe("Dashboard duplicate counts match the lists they link to", () => {
  it("reads the files it guards", () => {
    expect(dashboard.length).toBeGreaterThan(5000);
    expect(checker.length).toBeGreaterThan(5000);
    expect(refractJs.length).toBeGreaterThan(10000);
  });

  it("'Duplicate scenes' is the checker's own query at the checker's defaults", () => {
    // WHY: "Review" opens /sceneDuplicateChecker with no params, so the page
    // runs at its defaults. A count taken at any other distance or duration
    // difference stops matching the list it links to, with no error.
    const defaultDistance = checker.match(
      /query\.get\("distance"\) \?\? "(\d+)"/
    )?.[1];
    const defaultDuration = checker.match(
      /const defaultDurationDiff = "([\d.]+)"/
    )?.[1];
    const used = dashboard.match(
      /findDuplicateScenes\(distance:(\d+),duration_diff:([\d.]+)\)/
    );
    expect(
      used !== null,
      "the Dashboard no longer calls findDuplicateScenes"
    ).toBe(true);
    expect(
      used?.[1] === defaultDistance && used?.[2] === defaultDuration,
      `Dashboard counts at distance ${used?.[1]} / ${used?.[2]}s, the checker defaults to ${defaultDistance} / ${defaultDuration}s`
    ).toBe(true);

    const row = healthRow("Duplicate scenes");
    expect(row.includes("health.duplicates")).toBe(true);
    expect(row.includes('"/sceneDuplicateChecker"')).toBe(true);
  });

  it("'Scenes with duplicate files' counts and links with the same two clauses", () => {
    // WHY: a scene holding the same video twice is not a pair of duplicate
    // SCENES, so the checker cannot show it. This row must open the scenes
    // list, filtered by exactly what the count query filters by, or the number
    // and the list drift apart again (193 vs 84 was exactly that).
    const count =
      dashboard.match(/duplicateFiles: findScenes\([^\n]*\n/)?.[0] ?? "";
    expect(
      /duplicated:\{duplicated:true\}/.test(count) &&
        /file_count:\{modifier:GREATER_THAN,value:1\}/.test(count),
      "the duplicateFiles count query lost one of its two clauses"
    ).toBe(true);

    const row = healthRow("Scenes with duplicate files");
    expect(
      row.includes('makeCriterion("duplicated")') &&
        row.includes('makeCriterion("file_count")') &&
        row.includes("GQL.CriterionModifier.GreaterThan") &&
        /value: 1,/.test(row),
      "the row's link no longer builds the same two criteria as its count"
    ).toBe(true);
    expect(
      row.includes("/sceneDuplicateChecker"),
      "this row must not link to the duplicate checker: it cannot list these scenes"
    ).toBe(false);
  });
});

describe("duplicate checker banner: sets are sets, page figures say so", () => {
  // from the heading rewrite to where the banner is added to the page
  const start = refractJs.indexOf("Trim Stash's verbose");
  const end = refractJs.indexOf("panels.appendChild(summary)");
  const section = refractJs.slice(start, end);

  it("finds the section it guards", () => {
    expect(start).toBeGreaterThan(0);
    expect(end).toBeGreaterThan(start);
  });

  it("the set count is labelled 'sets', never 'duplicates'", () => {
    // WHY: the number comes from React's pager heading, "N sets of duplicates
    // found", which counts GROUPS of scenes. Rewriting it to "N duplicates"
    // made 84 sets (253 scenes) read as 84 duplicate scenes.
    expect(
      section.includes("</b> duplicates"),
      "the checker labels its set count 'duplicates' again"
    ).toBe(false);
    expect(section.includes("</b> sets")).toBe(true);
  });

  it("figures summed from one page say so when there are other pages", () => {
    // WHY: the table only holds one page of sets (20 by default), so the GB
    // and reclaimable figures cover that page alone. Unlabelled, "97 GB"
    // reads as the total across all 84 sets (it was 143 GB).
    expect(section.includes("totalSets > groups.length")).toBe(true);
    expect(
      (section.match(/on this page/g) ?? []).length >= 3,
      "the per-page figures are no longer labelled 'on this page'"
    ).toBe(true);
  });
});
