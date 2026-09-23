import React from "react";
import * as GQL from "src/core/generated-graphql";
import { SceneQueue } from "src/models/sceneQueue";
import { SceneCard } from "./SceneCard";
import {
  useCardWidth,
  useContainerDimensions,
} from "../Shared/GridCard/GridCard";

interface ISceneCardsGrid {
  scenes: GQL.SlimSceneDataFragment[];
  queue?: SceneQueue;
  selectedIds: Set<string>;
  zoomIndex: number;
  onSelectChange: (id: string, selected: boolean, shiftKey: boolean) => void;
  fromGroupId?: string;
}

// Preferred card widths per zoom level. calculateCardWidth divides the
// container by ceil(container / preferred), so raising these does not
// stretch cards at a given width — it drops a column and lets each card
// take the space back. That is what fixes the ultrawide case, where the
// old values packed 6 columns at 2560px and rendered cards SMALLER than
// at 1920px.
// Scenes default to zoom 2 (ListFilterModel forces defaultZoomIndex = 2),
// so that entry is the one that matters. 560 rather than 500: at 2560px
// the usable width is ~2520, and ceil(2520/500) is still 6 columns — the
// column count only drops to 5 at >=504. Measured: 500 -> 410px (6 col,
// unchanged and still smaller than at 1920px), 560 -> 494px (5 col).
// 1440px and 1920px are identical either way.
const zoomWidths = [300, 380, 560, 660];

export const SceneCardsGrid: React.FC<ISceneCardsGrid> = ({
  scenes,
  queue,
  selectedIds,
  zoomIndex,
  onSelectChange,
  fromGroupId,
}) => {
  const [componentRef, { width: containerWidth }] = useContainerDimensions();

  const cardWidth = useCardWidth(containerWidth, zoomIndex, zoomWidths);

  return (
    <div className="row justify-content-center" ref={componentRef}>
      {scenes.map((scene, index) => (
        <SceneCard
          key={scene.id}
          width={cardWidth}
          scene={scene}
          queue={queue}
          index={index}
          zoomIndex={zoomIndex}
          selecting={selectedIds.size > 0}
          selected={selectedIds.has(scene.id)}
          onSelectedChanged={(selected: boolean, shiftKey: boolean) =>
            onSelectChange(scene.id, selected, shiftKey)
          }
          fromGroupId={fromGroupId}
        />
      ))}
    </div>
  );
};
