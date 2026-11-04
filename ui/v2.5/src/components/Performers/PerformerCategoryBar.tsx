import React, { useMemo } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useConfigurationContext } from "src/hooks/Config";
import { IUIConfig, performerCategoryBarMax } from "src/core/config";
import { useFilter } from "../List/FilterProvider";
import {
  PerformerTagsCriterionOption,
  TagsCriterion,
  TagsCriterionOption,
} from "src/models/list-filter/criteria/tags";
import { ListFilterModel } from "src/models/list-filter/filter";

// A row of one-click category chips above the performers list, backed by tags
// the user picks in Settings > Interface.
//
// The chips drive the page's REAL filter rather than keeping their own state:
// the bar calls setFilter on the list's FilterContext, so the selection lands
// in the URL, survives a reload and a back button, and shows up in the normal
// filter UI where it can be refined or removed like any other criterion. A
// separate local filter would have silently disagreed with what the filter bar
// said was applied.
//
// PerformerCategoryBar MUST render inside ItemListContext - useFilter throws
// outside a FilterStateContext. See PerformerList's preListContent prop.
// The scenes list keeps its filter in local state instead (useFilteredItemList)
// and passes it to SceneCategoryBar directly.
//
// The scenes bar uses the same configured tags but filters on
// `performer_tags`: the categories describe performers, and a performer's tag
// is not inherited by their scenes (fork mod #30), so a plain scene `tags`
// filter would show only the few scenes tagged by hand.

// Chips union rather than intersect: selecting two categories shows performers
// in EITHER, not performers carrying both.
//
// This is not a style preference, it is what the data supports. Measured on
// this library: BBW has 47 performers and Bimbo 11, but the number carrying
// both is ZERO - performer tags describe a type, and a performer is rarely
// two types at once. With INCLUDES_ALL the second chip clicked would have
// emptied the page every time, which reads as a broken filter. INCLUDES gives
// 58, and is identical to INCLUDES_ALL when only one chip is active.
const CHIP_MODIFIER = GQL.CriterionModifier.Includes;

// Reads the tag IDs currently applied by the page's tags criterion. Only a
// criterion using the chips' own modifier counts: if the user has switched it
// by hand in the filter dialog (to Includes All, or Is Null) the chips stop
// claiming to represent it rather than mislabelling a filter they did not
// build.
function activeTagIDs(
  filter: ListFilterModel,
  criterionType: string
): Set<string> {
  const criterion = filter.criteria.find(
    (c) => c.criterionOption.type === criterionType
  ) as TagsCriterion | undefined;

  if (!criterion || criterion.modifier !== CHIP_MODIFIER) {
    return new Set();
  }

  return new Set(criterion.value.items.map((i) => i.id));
}

interface ICategoryBar {
  filter: ListFilterModel;
  setFilter: (filter: ListFilterModel) => void;
  criterionOption: typeof TagsCriterionOption;
}

const CategoryBar: React.FC<ICategoryBar> = ({
  filter,
  setFilter,
  criterionOption,
}) => {
  const { configuration } = useConfigurationContext();
  const criterionType = criterionOption.type;

  const configuredIDs = useMemo(() => {
    const ui = (configuration?.ui ?? {}) as IUIConfig;
    return (ui.performerCategoryTags ?? []).slice(0, performerCategoryBarMax);
  }, [configuration]);

  const { data } = GQL.useFindTagsForSelectQuery({
    variables: { ids: configuredIDs },
    skip: configuredIDs.length === 0,
  });

  // Sorted by name so the row reads like an index. A configured tag that has
  // since been deleted simply does not come back from the query and drops out
  // of the bar on its own - no stale chip pointing at a missing tag.
  const tags = useMemo(() => {
    const found = data?.findTags.tags ?? [];
    return [...found].sort((a, b) =>
      (a.sort_name || a.name).localeCompare(b.sort_name || b.name)
    );
  }, [data]);

  const active = useMemo(
    () => activeTagIDs(filter, criterionType),
    [filter, criterionType]
  );

  function apply(ids: string[], labels: Map<string, string>) {
    if (ids.length === 0) {
      // removeCriterion resets to page 1 itself
      setFilter(filter.removeCriterion(criterionType));
      return;
    }

    const criterion = new TagsCriterion(criterionOption);
    criterion.modifier = CHIP_MODIFIER;
    criterion.value = {
      items: ids.map((id) => ({ id, label: labels.get(id) ?? id })),
      excluded: [],
      depth: 0,
    };

    const next = filter.replaceCriteria(criterionType, [criterion]);
    // A narrowed list has fewer pages than the one being looked at, so
    // staying on page 7 would land on an empty list that looks like "no
    // performers in this category".
    next.currentPage = 1;
    setFilter(next);
  }

  const labels = useMemo(
    () => new Map(tags.map((t) => [t.id, t.name])),
    [tags]
  );

  function toggle(id: string) {
    const next = new Set(active);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    apply([...next], labels);
  }

  if (configuredIDs.length === 0 || tags.length === 0) {
    return null;
  }

  return (
    <div
      className="performer-category-bar"
      data-testid="performer-category-bar"
    >
      <Button
        className="performer-category-chip"
        variant={active.size === 0 ? "primary" : "secondary"}
        onClick={() => apply([], labels)}
      >
        <FormattedMessage id="config.ui.performer_category_bar.all" />
      </Button>
      {tags.map((tag) => (
        <Button
          key={tag.id}
          className="performer-category-chip"
          variant={active.has(tag.id) ? "primary" : "secondary"}
          onClick={() => toggle(tag.id)}
          title={tag.description ?? undefined}
        >
          {tag.name}
        </Button>
      ))}
    </div>
  );
};

export const PerformerCategoryBar: React.FC = () => {
  const { filter, setFilter } = useFilter();
  return (
    <CategoryBar
      filter={filter}
      setFilter={setFilter}
      criterionOption={TagsCriterionOption}
    />
  );
};

export const SceneCategoryBar: React.FC<{
  filter: ListFilterModel;
  setFilter: (filter: ListFilterModel) => void;
}> = ({ filter, setFilter }) => (
  <CategoryBar
    filter={filter}
    setFilter={setFilter}
    criterionOption={PerformerTagsCriterionOption}
  />
);

export default PerformerCategoryBar;
