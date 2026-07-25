import React, { useLayoutEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { OverlayTrigger, Popover } from "react-bootstrap";
import { useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";

type SlimPerformer = Pick<
  GQL.Performer,
  "id" | "name" | "disambiguation" | "gender" | "image_path"
>;

function getGenderClass(gender?: GQL.GenderEnum | null): string {
  switch (gender) {
    case GQL.GenderEnum.Male:
    case GQL.GenderEnum.TransgenderMale:
      return "gender-m";
    case GQL.GenderEnum.Female:
    case GQL.GenderEnum.TransgenderFemale:
      return "gender-f";
    case GQL.GenderEnum.Intersex:
    case GQL.GenderEnum.NonBinary:
      return "gender-n";
    default:
      return "";
  }
}

function stringToDate(dateString: string | null | undefined): Date | null {
  if (!dateString) return null;
  const parts = dateString.split("-");
  if (parts.length !== 3) return null;
  const year = Number(parts[0]);
  const monthIndex = Math.max(0, Number(parts[1]) - 1);
  const day = Number(parts[2]);
  return new Date(year, monthIndex, day, 0, 0, 0, 0);
}

function getAge(
  birthdate: string | null | undefined,
  fromDate: string | null | undefined
): number {
  if (!birthdate) return 0;
  const birth = stringToDate(birthdate);
  const from = fromDate ? stringToDate(fromDate) : new Date();
  if (!birth || !from) return 0;
  let age = from.getFullYear() - birth.getFullYear();
  if (
    birth.getMonth() > from.getMonth() ||
    (birth.getMonth() === from.getMonth() && birth.getDate() > from.getDate())
  ) {
    age -= 1;
  }
  return age;
}

const PerformerHoverPopover: React.FC<{
  performers: SlimPerformer[];
  sceneDate?: string | null;
}> = ({ performers, sceneDate }) => {
  const intl = useIntl();
  const { loading, data } = GQL.useFindPerformersQuery({
    variables: {
      performer_ids: performers.map((p) => parseInt(p.id, 10)),
    },
  });

  const getPerformerAge = (performer: SlimPerformer): string | null => {
    const performerResults = data?.findPerformers.performers;
    if (!performerResults || loading) return null;
    const p = performerResults.find((pr) => pr.id === performer.id);
    if (!p?.birthdate) return null;
    const age = getAge(p.birthdate, sceneDate ?? p.death_date);
    const yearsOld = intl.formatMessage({
      id: "years_old",
      defaultMessage: "years old",
    });
    if (sceneDate) {
      return intl.formatMessage(
        {
          id: "media_info.performer_card.age_context",
          defaultMessage: "{age} {years_old} at production",
        },
        { age, years_old: yearsOld }
      );
    }
    return intl.formatMessage(
      {
        id: "media_info.performer_card.age",
        defaultMessage: "{age} {years_old}",
      },
      { age, years_old: yearsOld }
    );
  };

  return (
    <div className="performer-popover-content">
      {loading ? (
        <div className="qx-loading-indicator">
          <LoadingIndicator />
        </div>
      ) : (
        performers.map((p) => (
          <Link to={`/performers/${p.id}`} key={p.id}>
            <div className="performer-row">
              <img
                className="image-thumbnail"
                alt={p.name ?? ""}
                src={p.image_path ?? ""}
              />
              <div className="performer-details">
                <div className={`name ${getGenderClass(p.gender)}`}>
                  {p.name}
                </div>
                <div className="age">{getPerformerAge(p)}</div>
              </div>
            </div>
          </Link>
        ))
      )}
    </div>
  );
};

export const SceneCardPerformerList: React.FC<{
  performers: SlimPerformer[];
  sceneDate?: string | null;
}> = ({ performers, sceneDate }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const moreRef = useRef<HTMLSpanElement>(null);
  const [visibleCount, setVisibleCount] = useState(performers.length);

  useLayoutEffect(() => {
    if (!containerRef.current || !performers.length) return;

    const calculateVisible = () => {
      if (!containerRef.current) return;
      const containerWidth = containerRef.current.offsetWidth;
      // If the container hasn't been laid out yet (0 width — e.g. measured
      // before Refract's overlay resolves), keep showing all names rather than
      // collapsing to just "+N". Collapsing to 0 removes the name children from
      // the DOM, leaving the ResizeObserver nothing to re-measure — a state it
      // can never recover from. Bail until a real width is available.
      if (!containerWidth) return;
      const elements = Array.from(
        containerRef.current.children
      ) as HTMLElement[];
      const moreWidth = moreRef.current?.offsetWidth || 10;
      let totalWidth = 0;
      let count = 0;

      for (const element of elements) {
        const elementWidth = element.offsetWidth;
        if (count < performers.length - 1) {
          if (totalWidth + elementWidth + moreWidth <= containerWidth) {
            totalWidth += elementWidth;
            count++;
          } else {
            break;
          }
        } else {
          if (totalWidth + elementWidth <= containerWidth) {
            totalWidth += elementWidth;
            count++;
          }
        }
      }
      // Never hide the only/first name: if nothing "fit" (e.g. a wide display
      // font vs a narrow measured container), still show one — overflow:hidden
      // clips it gracefully instead of the whole list vanishing into "+N".
      if (count === 0 && performers.length > 0) count = 1;
      setVisibleCount(count);
    };

    const resizeObserver = new ResizeObserver(() => calculateVisible());
    resizeObserver.observe(containerRef.current);
    calculateVisible();

    return () => resizeObserver.disconnect();
  }, [performers]);

  if (!performers.length) return null;

  const hiddenCount = performers.length - visibleCount;

  return (
    <div className="performers">
      <div className="list" ref={containerRef}>
        {performers.slice(0, visibleCount).map((p) => (
          <OverlayTrigger
            key={p.id}
            placement="bottom"
            trigger={["hover", "focus"]}
            overlay={
              <Popover id={`performer-popover-${p.id}`} className="performer-popover-container">
                <PerformerHoverPopover
                  performers={[p]}
                  sceneDate={sceneDate}
                />
              </Popover>
            }
          >
            <span className="performer-name">
              <Link
                to={`/performers/${p.id}`}
                className={getGenderClass(p.gender)}
              >
                {p.name}
              </Link>
            </span>
          </OverlayTrigger>
        ))}
      </div>
      {hiddenCount > 0 ? (
        <OverlayTrigger
          placement="bottom"
          trigger="click"
          overlay={
            <Popover id="performer-popover-more" className="performer-popover-container">
              <PerformerHoverPopover
                performers={performers.slice(visibleCount)}
                sceneDate={sceneDate}
              />
            </Popover>
          }
          rootClose
        >
          <span className="show-more" ref={moreRef}>
            +{hiddenCount}
          </span>
        </OverlayTrigger>
      ) : (
        <span className="show-more" ref={moreRef}>
          {" "}
        </span>
      )}
    </div>
  );
};
