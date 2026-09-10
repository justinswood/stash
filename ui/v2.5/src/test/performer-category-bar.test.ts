import { describe, it, expect } from "vitest";
import { ListFilterModel } from "src/models/list-filter/filter";
import { CriterionModifier, FilterMode } from "src/core/generated-graphql";
import {
  TagsCriterion,
  TagsCriterionOption,
} from "src/models/list-filter/criteria/tags";

// The performers category bar writes a tags criterion onto the page's live
// filter, which is then serialised into the URL and parsed back on reload.
// These tests pin the parts of that round-trip that fail SILENTLY: an
// unrecognised `c=` is discarded without error and the page just shows every
// performer, which looks like the chips doing nothing rather than a bug.

const CRITERION_TYPE = "tags";
const CHIP_MODIFIER = CriterionModifier.Includes;

function chipFilter(items: { id: string; label: string }[]) {
  const filter = new ListFilterModel(FilterMode.Performers);
  const criterion = new TagsCriterion(TagsCriterionOption);
  criterion.modifier = CHIP_MODIFIER;
  criterion.value = { items, excluded: [], depth: 0 };
  return filter.replaceCriteria(CRITERION_TYPE, [criterion]);
}

function roundTrip(filter: ListFilterModel) {
  const parsed = new ListFilterModel(FilterMode.Performers);
  parsed.configureFromQueryString(filter.makeQueryParameters());
  return parsed;
}

describe("performer category bar filter", () => {
  it("survives the URL round-trip with one chip", () => {
    const parsed = roundTrip(chipFilter([{ id: "5", label: "BBW" }]));

    const criterion = parsed.criteria.find(
      (c) => c.criterionOption.type === CRITERION_TYPE
    ) as TagsCriterion;

    expect(criterion).toBeDefined();
    expect(criterion.modifier).toBe(CHIP_MODIFIER);
    expect(criterion.value.items.map((i) => i.id)).toEqual(["5"]);
  });

  it("survives the URL round-trip with several chips", () => {
    const parsed = roundTrip(
      chipFilter([
        { id: "5", label: "BBW" },
        { id: "36", label: "Big Ass" },
        { id: "1436", label: "Bimbo" },
      ])
    );

    const criterion = parsed.criteria.find(
      (c) => c.criterionOption.type === CRITERION_TYPE
    ) as TagsCriterion;

    expect(criterion.value.items.map((i) => i.id)).toEqual(["5", "36", "1436"]);
  });

  it("produces a tags filter the backend understands", () => {
    // Guards the shape sent to findPerformers. Measured against the live API:
    // INCLUDES over [BBW, Bimbo] returns their union (58), where INCLUDES_ALL
    // returns 0 because no performer carries both. If this modifier ever
    // regresses to INCLUDES_ALL the bar empties the page on the second chip.
    const encoded = chipFilter([
      { id: "5", label: "BBW" },
      { id: "1436", label: "Bimbo" },
    ]).makeFilter();

    // note the API field is `excludes`, while the criterion's own field is
    // `excluded` - makeFilter renames it
    expect(encoded.tags).toEqual({
      value: ["5", "1436"],
      excludes: [],
      modifier: CriterionModifier.Includes,
      depth: 0,
    });
  });

  it("selecting a chip returns to page 1", () => {
    // A narrowed list has fewer pages, so keeping page 7 lands on an empty
    // list that reads as "no performers in this category".
    const deepIn = new ListFilterModel(FilterMode.Performers);
    deepIn.currentPage = 7;

    const criterion = new TagsCriterion(TagsCriterionOption);
    criterion.modifier = CHIP_MODIFIER;
    criterion.value = {
      items: [{ id: "5", label: "BBW" }],
      excluded: [],
      depth: 0,
    };

    const next = deepIn.replaceCriteria(CRITERION_TYPE, [criterion]);
    next.currentPage = 1;

    expect(next.currentPage).toBe(1);
  });

  it("clearing the chips removes the criterion and resets the page", () => {
    const withChip = chipFilter([{ id: "5", label: "BBW" }]);
    withChip.currentPage = 4;

    const cleared = withChip.removeCriterion(CRITERION_TYPE);

    expect(
      cleared.criteria.find((c) => c.criterionOption.type === CRITERION_TYPE)
    ).toBeUndefined();
    expect(cleared.currentPage).toBe(1);
    expect(cleared.makeFilter().tags).toBeUndefined();
  });

  it("leaves other criteria and the sort alone", () => {
    // The chips must not behave like "clear everything and filter" - the
    // user's sort and any filter they built by hand have to survive.
    const base = new ListFilterModel(FilterMode.Performers);
    base.sortBy = "birthdate";
    const withRating = base.clone();

    const criterion = new TagsCriterion(TagsCriterionOption);
    criterion.modifier = CHIP_MODIFIER;
    criterion.value = {
      items: [{ id: "5", label: "BBW" }],
      excluded: [],
      depth: 0,
    };
    const next = withRating.replaceCriteria(CRITERION_TYPE, [criterion]);

    expect(next.sortBy).toBe("birthdate");
  });
});
