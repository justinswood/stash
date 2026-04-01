import React, { useMemo } from "react";
import { FormattedMessage, FormattedDate, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { SceneCard } from "src/components/Scenes/SceneCard";
import TextUtils from "src/utils/text";
import "./History.scss";

const ITEMS_PER_PAGE = 100;

export const History: React.FC = () => {
  const intl = useIntl();

  const { data, loading, error } = GQL.useFindScenesQuery({
    variables: {
      filter: {
        sort: "last_played_at",
        direction: GQL.SortDirectionEnum.Desc,
        per_page: ITEMS_PER_PAGE,
        page: 1,
      },
      scene_filter: {
        play_count: {
          value: 0,
          modifier: GQL.CriterionModifier.GreaterThan,
        },
      },
    },
  });

  const groupedScenes = useMemo(() => {
    if (!data?.findScenes.scenes) return [];

    const groups: { date: string; scenes: typeof data.findScenes.scenes }[] =
      [];
    let currentDate = "";

    for (const scene of data.findScenes.scenes) {
      const playedAt = scene.last_played_at;
      const dateStr = playedAt
        ? new Date(playedAt).toLocaleDateString()
        : "Unknown";

      if (dateStr !== currentDate) {
        currentDate = dateStr;
        groups.push({ date: playedAt ?? "", scenes: [] });
      }
      groups[groups.length - 1].scenes.push(scene);
    }

    return groups;
  }, [data]);

  if (loading) return <LoadingIndicator />;
  if (error) return <span>{error.message}</span>;

  const totalCount = data?.findScenes.count ?? 0;
  const totalDuration = data?.findScenes.scenes.reduce(
    (sum, s) => sum + (s.play_duration ?? 0),
    0
  );

  return (
    <div className="history-page mt-3">
      <div className="history-header">
        <h2>
          <FormattedMessage id="watch_history" defaultMessage="Watch History" />
        </h2>
        <div className="history-stats">
          <span>
            {totalCount}{" "}
            <FormattedMessage
              id="scenes_watched"
              defaultMessage="scenes watched"
            />
          </span>
          {totalDuration ? (
            <span className="ml-3">
              {TextUtils.secondsAsTimeString(totalDuration, 2)}{" "}
              <FormattedMessage
                id="total_watch_time"
                defaultMessage="total watch time"
              />
            </span>
          ) : null}
        </div>
      </div>

      {groupedScenes.length === 0 && (
        <div className="history-empty">
          <p>
            <FormattedMessage
              id="no_watch_history"
              defaultMessage="No watch history yet. Start playing scenes to see them here."
            />
          </p>
        </div>
      )}

      {groupedScenes.map((group, idx) => (
        <div key={idx} className="history-group">
          <h5 className="history-date">
            {group.date ? (
              <FormattedDate
                value={group.date}
                year="numeric"
                month="long"
                day="numeric"
              />
            ) : (
              "Unknown"
            )}
          </h5>
          <div className="history-cards">
            {group.scenes.map((scene) => (
              <SceneCard
                key={scene.id}
                scene={scene}
                index={0}
                queue={undefined}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
};

export default History;
