import React from "react";
import { Helmet } from "react-helmet";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";
import { useTitleProps } from "src/hooks/title";
import * as GQL from "src/core/generated-graphql";
import { TagCardGrid } from "../Tags/TagCardGrid";
import { LoadingIndicator } from "../Shared/LoadingIndicator";

// The Categories hub lists the child tags of a single parent tag literally
// named "Categories". Curate the grid by parenting tags under it in the normal
// tag editor — no code change required to add or remove a category.
const CATEGORY_PARENT_NAME = "Categories";

const noop = () => {};
const emptySelection = new Set<string>();

const CategoriesGrid: React.FC<{ parentId: string }> = ({ parentId }) => {
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

  if (loading) {
    return <LoadingIndicator />;
  }

  const tags = data?.findTags.tags ?? [];

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
    <TagCardGrid
      tags={tags}
      zoomIndex={1}
      selectedIds={emptySelection}
      onSelectChange={noop}
    />
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
    <div className="container-fluid mt-4">
      <h1 className="mb-4">
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
