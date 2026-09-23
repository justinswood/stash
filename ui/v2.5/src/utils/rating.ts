export enum RatingSystemType {
  Stars = "stars",
  Decimal = "decimal",
}

export enum RatingStarPrecision {
  Full = "full",
  Half = "half",
  Quarter = "quarter",
  Tenth = "tenth",
}

// Decimal (0.1-10.0) rather than Stars. Ratings read as numbers everywhere.
// This is the DEFAULT, so it applies only to accounts with no stored
// ratingSystemOptions — anyone who has used Settings -> Interface -> Rating
// system keeps their choice. defaultRatingStarPrecision deliberately stays
// Full: it is unread in decimal mode, and changing it would alter
// RatingBanner's isLegacy branch for anyone who switches back to stars.
export const defaultRatingSystemType: RatingSystemType =
  RatingSystemType.Decimal;
export const defaultRatingStarPrecision: RatingStarPrecision =
  RatingStarPrecision.Full;

export const ratingSystemIntlMap = new Map<RatingSystemType, string>([
  [
    RatingSystemType.Stars,
    "config.ui.editing.rating_system.type.options.stars",
  ],
  [
    RatingSystemType.Decimal,
    "config.ui.editing.rating_system.type.options.decimal",
  ],
]);

export const ratingStarPrecisionIntlMap = new Map<RatingStarPrecision, string>([
  [
    RatingStarPrecision.Full,
    "config.ui.editing.rating_system.star_precision.options.full",
  ],
  [
    RatingStarPrecision.Half,
    "config.ui.editing.rating_system.star_precision.options.half",
  ],
  [
    RatingStarPrecision.Quarter,
    "config.ui.editing.rating_system.star_precision.options.quarter",
  ],
  [
    RatingStarPrecision.Tenth,
    "config.ui.editing.rating_system.star_precision.options.tenth",
  ],
]);

export type RatingSystemOptions = {
  type: RatingSystemType;
  starPrecision?: RatingStarPrecision;
};

export const defaultRatingSystemOptions = {
  type: defaultRatingSystemType,
  starPrecision: defaultRatingStarPrecision,
};

function round(value: number, step: number) {
  let denom = step;
  if (!denom) {
    denom = 1.0;
  }
  const inv = 1.0 / denom;
  return Math.round(value * inv) / inv;
}

export function getRatingPrecision(precision: RatingStarPrecision) {
  switch (precision) {
    case RatingStarPrecision.Full:
      return 1;
    case RatingStarPrecision.Half:
      return 0.5;
    case RatingStarPrecision.Quarter:
      return 0.25;
    case RatingStarPrecision.Tenth:
      return 0.1;
    default:
      return 1;
  }
}

export function convertToRatingFormat(
  rating: number | null | undefined,
  ratingSystemOptions: RatingSystemOptions
) {
  if (!rating) {
    return null;
  }

  const { type, starPrecision } = ratingSystemOptions;

  const precision =
    type === RatingSystemType.Decimal
      ? 0.1
      : getRatingPrecision(starPrecision ?? RatingStarPrecision.Full);
  const maxValue = type === RatingSystemType.Decimal ? 10 : 5;
  const denom = 100 / maxValue;

  return round(rating / denom, precision);
}

export function convertFromRatingFormat(
  rating: number,
  ratingSystem: RatingSystemType | undefined
) {
  const maxValue =
    (ratingSystem ?? RatingSystemType.Stars) === RatingSystemType.Decimal
      ? 10
      : 5;
  const factor = 100 / maxValue;

  return Math.round(rating * factor);
}
