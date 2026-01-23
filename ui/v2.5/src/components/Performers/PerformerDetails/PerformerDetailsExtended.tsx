import React, { useMemo } from "react";
import { gql, useQuery } from "@apollo/client";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import TextUtils from "src/utils/text";
import { DetailItem } from "src/components/Shared/DetailItem";

const maxPartnerLinks = 3;

const performerDetailsExtendedQuery = gql`
  query PerformerDetailsExtended($performer_id: ID!) {
    performerDetailsExtended(performer_id: $performer_id) {
      top_male_partners {
        id
        name
      }
      top_male_partner_count
      top_female_partners {
        id
        name
      }
      top_female_partner_count
      total_content_time
      scenes_total
      scenes_organized
      scenes_timespan {
        earliest_date
        latest_date
      }
    }
  }
`;

interface IPerformerDetailsExtendedProps {
  performer: GQL.PerformerDataFragment;
  collapsed?: boolean;
}

type PerformerDetailsExtendedResponse = {
  performerDetailsExtended: {
    top_male_partners: Array<{ id: string; name: string }>;
    top_male_partner_count: number;
    top_female_partners: Array<{ id: string; name: string }>;
    top_female_partner_count: number;
    total_content_time: number;
    scenes_total: number;
    scenes_organized: number;
    scenes_timespan: {
      earliest_date?: string | null;
      latest_date?: string | null;
    } | null;
  };
};

type PerformerDetailsExtendedVars = {
  performer_id: string;
};

export const PerformerDetailsExtended: React.FC<
  IPerformerDetailsExtendedProps
> = ({ performer, collapsed }) => {
  const intl = useIntl();

  const { data, loading } = useQuery<
    PerformerDetailsExtendedResponse,
    PerformerDetailsExtendedVars
  >(performerDetailsExtendedQuery, {
    variables: { performer_id: performer.id },
    skip: !!collapsed,
  });

  const details = data?.performerDetailsExtended;

  const {
    totalScenes,
    organizedScenes,
    organizedPercent,
    timespan,
    totalDuration,
    topMalePartners,
    topFemalePartners,
    malePartnerCount,
    femalePartnerCount,
  } = useMemo(() => {
    const result = {
      totalScenes: 0,
      organizedScenes: 0,
      organizedPercent: 0,
      timespan: undefined as string | undefined,
      totalDuration: 0,
      topMalePartners: [] as PerformerDetailsExtendedResponse["performerDetailsExtended"]["top_male_partners"],
      topFemalePartners: [] as PerformerDetailsExtendedResponse["performerDetailsExtended"]["top_female_partners"],
      malePartnerCount: 0,
      femalePartnerCount: 0,
    };

    if (!details) return result;

    result.totalScenes = details.scenes_total ?? 0;
    result.totalDuration = details.total_content_time ?? 0;
    result.organizedScenes = details.scenes_organized ?? 0;
    result.organizedPercent =
      result.totalScenes > 0
        ? Math.round((result.organizedScenes / result.totalScenes) * 100)
        : 0;

    const earliest = details.scenes_timespan?.earliest_date ?? undefined;
    const latest = details.scenes_timespan?.latest_date ?? undefined;
    if (earliest && latest) {
      result.timespan = `${TextUtils.formatFuzzyDate(
        intl,
        earliest
      )} – ${TextUtils.formatFuzzyDate(intl, latest)}`;
    }

    result.topMalePartners = details.top_male_partners ?? [];
    result.topFemalePartners = details.top_female_partners ?? [];
    result.malePartnerCount = details.top_male_partner_count ?? 0;
    result.femalePartnerCount = details.top_female_partner_count ?? 0;

    return result;
  }, [details, intl]);

  if (collapsed || loading || !details) {
    return null;
  }

  const renderPartners = (
    partners: PerformerDetailsExtendedResponse["performerDetailsExtended"]["top_male_partners"],
    count: number
  ) => {
    if (!partners.length) return undefined;

    const limited = partners.slice(0, maxPartnerLinks);
    const remaining = partners.length - limited.length;

    return (
      <span className="detail-item-list">
        {limited.map((partner, index) => (
          <React.Fragment key={partner.id}>
            <Link to={`/performers/${partner.id}`}>{partner.name}</Link>
            {index < limited.length - 1 ? ", " : ""}
          </React.Fragment>
        ))}
        {remaining > 0 ? `, and ${remaining} more` : ""}
        <span className="ml-2 text-muted">
          {count} {count === 1 ? "scene" : "scenes"}
        </span>
      </span>
    );
  };

  const organizedValue =
    totalScenes > 0
      ? `${organizedPercent}% of ${totalScenes}`
      : undefined;

  return (
    <div className="detail-group performer-details-extended">
      <DetailItem
        id="appears-most-with-male"
        label="Appears Most With (Male)"
        value={renderPartners(topMalePartners, malePartnerCount)}
        showEmpty={false}
      />
      <DetailItem
        id="appears-most-with-female"
        label="Appears Most With (Female)"
        value={renderPartners(topFemalePartners, femalePartnerCount)}
        showEmpty={false}
      />
      <DetailItem
        id="total-content-time"
        label="Total Content Time"
        value={
          totalDuration
            ? TextUtils.secondsAsTimeString(totalDuration, 3)
            : undefined
        }
        showEmpty={false}
      />
      <DetailItem
        id="scenes-organized-percent"
        label="Scenes Organized"
        value={organizedValue}
        showEmpty={false}
      />
      <DetailItem
        id="scenes-timespan"
        label="Scenes Timespan"
        value={timespan}
        showEmpty={false}
      />
    </div>
  );
};

export default PerformerDetailsExtended;
