import { Button, Dropdown } from "react-bootstrap";
import React, { useState } from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { SubmitStashBoxDraft } from "src/components/Dialogs/SubmitDraft";

interface IPerformerOperationsProps {
  performer: GQL.PerformerDataFragment;
  // Render as an item in an existing dropdown - the performer sidebar's More
  // menu - instead of a standalone button. The draft modal renders either way.
  asDropdownItem?: boolean;
}

export const PerformerSubmitButton: React.FC<IPerformerOperationsProps> = ({
  performer,
  asDropdownItem,
}) => {
  const [showDraftModal, setShowDraftModal] = useState(false);

  const { data } = GQL.useConfigurationQuery();
  const boxes = data?.configuration?.general?.stashBoxes ?? [];

  if (boxes.length === 0) return null;

  const label = <FormattedMessage id="actions.submit_stash_box" />;

  return (
    <>
      {asDropdownItem ? (
        <Dropdown.Item onClick={() => setShowDraftModal(true)}>
          {label}
        </Dropdown.Item>
      ) : (
        <Button onClick={() => setShowDraftModal(true)}>{label}</Button>
      )}
      <SubmitStashBoxDraft
        type="performer"
        boxes={boxes}
        entity={performer}
        show={showDraftModal}
        onHide={() => setShowDraftModal(false)}
      />
    </>
  );
};
