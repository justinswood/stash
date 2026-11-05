import React from "react";
import { Form } from "react-bootstrap";

export interface IRatingSelectProps {
  value: number | null | undefined;
  onSetRating: (value: number | null) => void;
}

const WHOLE_RATINGS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

// A 1-10 dropdown in place of the typed decimal rating on the scene and
// performer pages. Ratings are stored as rating100, so N is saved as N * 10.
// A rating set elsewhere that is not a whole number (e.g. 8.5 = 85) gets its
// own option, so opening the page never silently shows or saves a different
// value.
export const RatingSelect: React.FC<IRatingSelectProps> = ({
  value,
  onSetRating,
}) => {
  const current = value ?? null;
  const offGrid = current !== null && current % 10 !== 0;

  return (
    <Form.Control
      as="select"
      className="rating-select"
      aria-label="Rating"
      value={current === null ? "" : String(current)}
      onChange={(e) => {
        const v = e.currentTarget.value;
        onSetRating(v === "" ? null : Number(v));
      }}
    >
      <option value="">★ –</option>
      {offGrid && (
        <option value={String(current)}>★ {(current / 10).toFixed(1)}</option>
      )}
      {WHOLE_RATINGS.map((n) => (
        <option key={n} value={String(n * 10)}>
          ★ {n}
        </option>
      ))}
    </Form.Control>
  );
};
