import { describe, it, expect } from "vitest";
import { ListFilterModel } from "src/models/list-filter/filter";
import {
  FilterMode,
  SortDirectionEnum,
} from "src/core/generated-graphql";

describe("ListFilterModel", () => {
  it("initializes with correct defaults for scenes", () => {
    const filter = new ListFilterModel(FilterMode.Scenes);

    expect(filter.mode).toBe(FilterMode.Scenes);
    expect(filter.currentPage).toBe(1);
    expect(filter.itemsPerPage).toBe(40);
    expect(filter.sortBy).toBe("date");
    expect(filter.sortDirection).toBe(SortDirectionEnum.Desc);
    expect(filter.criteria).toHaveLength(0);
    expect(filter.searchTerm).toBe("");
  });

  it("initializes with correct defaults for performers", () => {
    const filter = new ListFilterModel(FilterMode.Performers);

    expect(filter.mode).toBe(FilterMode.Performers);
    expect(filter.sortBy).toBe("name");
  });

  it("respects custom default sort options", () => {
    const filter = new ListFilterModel(FilterMode.Scenes, undefined, {
      defaultSortBy: "created_at",
      defaultSortDir: SortDirectionEnum.Asc,
    });

    expect(filter.sortBy).toBe("created_at");
    expect(filter.sortDirection).toBe(SortDirectionEnum.Asc);
  });

  it("generates correct FindFilterType", () => {
    const filter = new ListFilterModel(FilterMode.Scenes);
    filter.currentPage = 2;
    filter.itemsPerPage = 25;
    filter.sortBy = "last_played_at";
    filter.sortDirection = SortDirectionEnum.Desc;

    const findFilter = filter.makeFindFilter();

    expect(findFilter.page).toBe(2);
    expect(findFilter.per_page).toBe(25);
    expect(findFilter.sort).toBe("last_played_at");
    expect(findFilter.direction).toBe(SortDirectionEnum.Desc);
  });

  it("generates empty filter when no criteria", () => {
    const filter = new ListFilterModel(FilterMode.Scenes);
    const output = filter.makeFilter();

    expect(output).toEqual({});
  });

  it("tracks criteria count correctly", () => {
    const filter = new ListFilterModel(FilterMode.Scenes);
    expect(filter.criteria.length).toBe(0);
  });

  it("clones without shared references", () => {
    const filter = new ListFilterModel(FilterMode.Scenes);
    filter.searchTerm = "test";
    filter.currentPage = 3;

    const clone = filter.clone();

    expect(clone.searchTerm).toBe("test");
    expect(clone.currentPage).toBe(3);
    expect(clone.mode).toBe(FilterMode.Scenes);

    // Modifying clone shouldn't affect original
    clone.searchTerm = "modified";
    clone.currentPage = 5;
    expect(filter.searchTerm).toBe("test");
    expect(filter.currentPage).toBe(3);
  });
});
