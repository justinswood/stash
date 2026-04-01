import { describe, it, expect } from "vitest";
import {
  FilterMode,
  SortDirectionEnum,
} from "src/core/generated-graphql";
import {
  generateDefaultFrontPageContent,
  generatePremadeFrontPageContent,
  ICustomFilter,
} from "src/core/config";

// Minimal intl mock
const mockIntl = {
  formatMessage: ({ id }: { id: string }) => id,
} as any;

describe("FrontPage content generation", () => {
  it("generates default front page content", () => {
    const content = generateDefaultFrontPageContent(mockIntl);

    expect(content.length).toBeGreaterThan(0);
    expect(content[0].__typename).toBe("CustomFilter");
  });

  it("includes Continue Watching as first item in defaults", () => {
    const content = generateDefaultFrontPageContent(mockIntl);
    const first = content[0] as ICustomFilter;

    expect(first.message?.id).toBe("continue_watching");
    expect(first.mode).toBe(FilterMode.Scenes);
    expect(first.sortBy).toBe("last_played_at");
    expect(first.direction).toBe(SortDirectionEnum.Desc);
    expect(first.applyCriteria).toBeTypeOf("function");
  });

  it("includes Recently Added Scenes in defaults", () => {
    const content = generateDefaultFrontPageContent(mockIntl);
    const recentlyAdded = content.find(
      (c) =>
        c.__typename === "CustomFilter" &&
        (c as ICustomFilter).sortBy === "created_at" &&
        (c as ICustomFilter).mode === FilterMode.Scenes
    );

    expect(recentlyAdded).toBeDefined();
  });

  it("generates premade content with all entity types", () => {
    const content = generatePremadeFrontPageContent(mockIntl);
    const modes = content
      .filter((c) => c.__typename === "CustomFilter")
      .map((c) => (c as ICustomFilter).mode);

    expect(modes).toContain(FilterMode.Scenes);
    expect(modes).toContain(FilterMode.Galleries);
    expect(modes).toContain(FilterMode.Groups);
    expect(modes).toContain(FilterMode.Studios);
    expect(modes).toContain(FilterMode.Performers);
  });

  it("premade content includes Continue Watching", () => {
    const content = generatePremadeFrontPageContent(mockIntl);
    const cw = content.find(
      (c) =>
        c.__typename === "CustomFilter" &&
        (c as ICustomFilter).message?.id === "continue_watching"
    );

    expect(cw).toBeDefined();
  });
});
