import React from "react";
import * as GQL from "src/core/generated-graphql";
import { FilteredSceneList } from "src/components/Scenes/SceneList";
import { usePerformerTagFilterHook } from "src/core/tags";
import { View } from "src/components/List/views";

interface ITagPerformerScenesPanel {
  active: boolean;
  tag: GQL.TagDataFragment;
  showSubTagContent?: boolean;
}

// Scenes reached through the tag's *performers* rather than the tag on the
// scene itself. See TagScenesPanel for the latter.
export const TagPerformerScenesPanel: React.FC<ITagPerformerScenesPanel> = ({
  active,
  tag,
  showSubTagContent,
}) => {
  const filterHook = usePerformerTagFilterHook(tag, showSubTagContent);
  return (
    <FilteredSceneList
      filterHook={filterHook}
      alterQuery={active}
      view={View.TagPerformerScenes}
    />
  );
};
