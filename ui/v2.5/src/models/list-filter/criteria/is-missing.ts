import { CriterionModifier } from "src/core/generated-graphql";
import { CriterionType } from "../types";
import { ModifierCriterionOption, StringCriterion, Option } from "./criterion";

export class IsMissingCriterion extends StringCriterion {
  public toCriterionInput(): string {
    return this.value;
  }
}

class IsMissingCriterionOption extends ModifierCriterionOption {
  constructor(messageID: string, type: CriterionType, options: Option[]) {
    super({
      messageID,
      type,
      options,
      modifierOptions: [],
      defaultModifier: CriterionModifier.Equals,
      makeCriterion: () => new IsMissingCriterion(this),
    });
  }
}

export const SceneIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  [
    "title",
    "cover",
    "details",
    "url",
    "date",
    "galleries",
    "studio",
    "group",
    "performers",
    "tags",
    "stash_id",
    // The backend has always handled is_missing:"phash"
    // (fingerprints_phash.fingerprint IS NULL); it just wasn't offered here, so
    // the dashboard's "Pending phashes" link had no criterion to render.
    "phash",
  ]
);

export const ImageIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  ["title", "galleries", "studio", "performers", "tags"]
);

export const PerformerIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  [
    "url",
    "ethnicity",
    "country",
    "hair_color",
    "eye_color",
    "height",
    "weight",
    "measurements",
    "fake_tits",
    "career_length",
    "tattoos",
    "piercings",
    "aliases",
    "gender",
    "image",
    "details",
    "stash_id",
  ]
);

export const GalleryIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  ["title", "details", "url", "date", "studio", "performers", "tags", "scenes"]
);

export const TagIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  ["image"]
);

export const StudioIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  ["image", "stash_id", "details"]
);

export const GroupIsMissingCriterionOption = new IsMissingCriterionOption(
  "isMissing",
  "is_missing",
  ["front_image", "back_image", "scenes"]
);
