import React, { useMemo, useState } from "react";
import { Helmet } from "react-helmet";
import { FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { useTitleProps } from "src/hooks/title";
import * as GQL from "src/core/generated-graphql";
import { TagCard } from "../Tags/TagCard";
import { ClearableInput } from "../Shared/ClearableInput";
import { LoadingIndicator } from "../Shared/LoadingIndicator";

// The Categories hub lists the child tags of a single parent tag literally
// named "Categories". Curate the grid by parenting tags under it in the normal
// tag editor — no code change required to add or remove a category.
const CATEGORY_PARENT_NAME = "Categories";

const noop = () => {};

// Fixed card width — see the render comment below for why this page does not
// use TagCardGrid's measured sizing.
const CATEGORY_CARD_WIDTH = 300;

// A category describes the *performer*, so the useful scene figure is "scenes
// whose performers carry this tag", not "scenes carrying this tag". The tag row
// has no column for that, so each card counts it itself — a per_page: 0 query
// that returns the count without materialising any scenes (~3ms each).
//
// This also repoints the scene-count badge. Left alone it links to
// /scenes?c=(tags…), a filtered scene list with no tabs, which is a dead end
// from here: it shows the 2 directly-tagged scenes rather than the 52 reachable
// through the performers.
const CategoryCard: React.FC<{ tag: GQL.TagListDataFragment }> = ({ tag }) => {
  const performerScenesURL = `/tags/${tag.id}/performer-scenes`;

  const { data } = GQL.useFindScenesQuery({
    variables: {
      filter: { per_page: 0 },
      scene_filter: {
        performer_tags: {
          value: [tag.id],
          modifier: GQL.CriterionModifier.IncludesAll,
          depth: 0,
        },
      },
    },
  });

  return (
    <TagCard
      tag={tag}
      linkTo={performerScenesURL}
      sceneCount={data?.findScenes.count}
      sceneCountLinkTo={performerScenesURL}
      cardWidth={CATEGORY_CARD_WIDTH}
      zoomIndex={1}
      selecting={false}
      selected={false}
      onSelectedChanged={noop}
    />
  );
};

const CategoriesGrid: React.FC<{ parentId: string }> = ({ parentId }) => {
  const intl = useIntl();
  const [query, setQuery] = useState("");

  const { data, loading } = GQL.useFindTagsQuery({
    variables: {
      filter: { per_page: -1, sort: "name", direction: GQL.SortDirectionEnum.Asc },
      tag_filter: {
        parents: {
          value: [parentId],
          modifier: GQL.CriterionModifier.Includes,
          depth: 0,
        },
      },
    },
  });

  const tags = useMemo(() => data?.findTags.tags ?? [], [data]);

  // Filtered in the browser rather than re-queried: the page already fetches
  // every category up front (per_page: -1) and the list is small, so matching
  // locally is instant and costs no round-trip per keystroke.
  // Aliases are matched too — they're already in TagData, and a category is
  // often searched for by a name it isn't filed under.
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return tags;
    return tags.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        (t.aliases ?? []).some((a) => a.toLowerCase().includes(q))
    );
  }, [tags, query]);

  if (loading) {
    return <LoadingIndicator />;
  }

  if (tags.length === 0) {
    return (
      <div className="text-center mt-4">
        <FormattedMessage
          id="categories.no_children"
          defaultMessage="The “Categories” tag has no child tags yet. Parent tags under it to have them appear here."
        />
      </div>
    );
  }

  return (
    <>
      <div className="categories-toolbar mb-3 d-flex justify-content-center align-items-center">
        <ClearableInput
          value={query}
          setValue={setQuery}
          placeholder={`${intl.formatMessage({ id: "actions.search" })}…`}
        />
        {!!query.trim() && (
          <span className="categories-result-count text-muted">
            <FormattedMessage
              id="categories.match_count"
              defaultMessage="{count} of {total}"
              values={{ count: filtered.length, total: tags.length }}
            />
          </span>
        )}
      </div>

      {filtered.length === 0 ? (
        <div className="text-center mt-4">
          <FormattedMessage
            id="categories.no_matches"
            defaultMessage="No categories match “{query}”."
            values={{ query: query.trim() }}
          />
        </div>
      ) : (
        /* Cards are rendered directly at a fixed width rather than through
           TagCardGrid. That component sizes cards from a ResizeObserver on its
           own container (useContainerDimensions → useCardWidth), and here the
           card width feeds back into the measured height — `.tag-card-header`
           is `aspect-ratio: 5/3`, so height tracks width — which left the grid
           oscillating: cards rendered large, got measured, shrank, and round
           again, continuously.
           This page is a small curated list with no zoom slider, so there is
           nothing to measure for: a fixed width in a wrapping flex row gives a
           stable layout and removes the feedback path entirely. */
        <div className="row justify-content-center">
          {filtered.map((tag) => (
            <CategoryCard key={tag.id} tag={tag} />
          ))}
        </div>
      )}
    </>
  );
};

const CategoriesPage: React.FC = () => {
  // Resolve the parent "Categories" tag by name.
  const { data, loading } = GQL.useFindTagsQuery({
    variables: {
      filter: { per_page: 1 },
      tag_filter: {
        name: {
          value: CATEGORY_PARENT_NAME,
          modifier: GQL.CriterionModifier.Equals,
        },
      },
    },
  });

  const parent = data?.findTags.tags[0];

  return (
    <div className="container-fluid mt-4 categories-page">
      <h1 className="mb-4 text-center">
        <FormattedMessage id="categories" defaultMessage="Categories" />
      </h1>
      {loading ? (
        <LoadingIndicator />
      ) : !parent ? (
        <div className="text-center mt-4">
          <FormattedMessage
            id="categories.no_parent"
            defaultMessage="No tag named “Categories” was found. Create one and parent your genre tags under it to populate this page."
          />
        </div>
      ) : (
        <CategoriesGrid parentId={parent.id} />
      )}
    </div>
  );
};

const Categories: React.FC = () => {
  const titleProps = useTitleProps({ id: "categories" });
  return (
    <>
      <Helmet {...titleProps} />
      <CategoriesPage />
    </>
  );
};

export default Categories;
