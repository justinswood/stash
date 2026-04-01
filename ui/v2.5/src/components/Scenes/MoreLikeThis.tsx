import React from "react";
import * as GQL from "src/core/generated-graphql";
import { SceneCard } from "./SceneCard";
import { FormattedMessage } from "react-intl";

interface IProps {
  scene: Pick<
    GQL.SceneDataFragment,
    "id" | "performers" | "tags" | "studio"
  >;
}

export const MoreLikeThis: React.FC<IProps> = ({ scene }) => {
  // Build filter: scenes sharing performers OR tags with this scene, excluding this scene
  const performerIds = scene.performers?.map((p) => p.id) ?? [];
  const tagIds = scene.tags?.map((t) => t.id) ?? [];
  const studioId = scene.studio?.id;

  // Prefer performers, fall back to tags, then studio
  const hasFilter = performerIds.length > 0 || tagIds.length > 0 || studioId;

  const sceneFilter: GQL.SceneFilterType = {};

  if (performerIds.length > 0) {
    sceneFilter.performers = {
      value: performerIds,
      modifier: GQL.CriterionModifier.IncludesAll,
    };
  } else if (tagIds.length > 0) {
    sceneFilter.tags = {
      value: tagIds.slice(0, 5), // Limit to first 5 tags
      modifier: GQL.CriterionModifier.Includes,
    };
  } else if (studioId) {
    sceneFilter.studios = {
      value: [studioId],
      modifier: GQL.CriterionModifier.Includes,
      depth: 0,
    };
  }

  const { data, loading } = GQL.useFindScenesQuery({
    skip: !hasFilter,
    variables: {
      filter: {
        sort: "random",
        per_page: 12,
        page: 1,
      },
      scene_filter: sceneFilter,
    },
  });

  // Filter out the current scene and check we have results
  const scenes =
    data?.findScenes.scenes.filter((s) => s.id !== scene.id) ?? [];

  if (loading || scenes.length === 0) return null;

  return (
    <div className="more-like-this mt-4">
      <h5>
        <FormattedMessage
          id="more_like_this"
          defaultMessage="More Like This"
        />
      </h5>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))",
          gap: "1rem",
        }}
      >
        {scenes.slice(0, 6).map((s) => (
          <SceneCard key={s.id} scene={s} index={0} queue={undefined} />
        ))}
      </div>
    </div>
  );
};
