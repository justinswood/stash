import React, { useState } from "react";
import { Button, Dropdown, Modal } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { ShareButton } from "src/components/ShareLinks/ShareButton";
import { PerformerImageSearchButton } from "./PerformerImageSearchButton";
import { PerformerSubmitButton } from "./PerformerSubmitButton";

interface IProps {
  performer: GQL.PerformerDataFragment;
  onEdit: () => void;
  onAutoTag: () => void;
  onMerge: () => void;
  onDelete: () => void;
}

// The performer sidebar's action row: one primary button, Edit, and every other
// action in a More menu. The old header laid seven buttons out flat, which in a
// 280px column would wrap onto three lines.
//
// The menu is positioned with a FIXED popper. The sidebar scrolls on its own
// (overflow-y: auto), so an absolutely positioned menu inside it would be
// clipped at the sidebar's edge. Fixed positioning escapes that clip - but only
// because the sidebar does not carry the theme's .detail-header class, which
// sets backdrop-filter. A backdrop-filter, filter or transform on any ancestor
// makes it the containing block for fixed descendants, which puts the menu
// straight back inside the clip.
export const PerformerSidebarActions: React.FC<IProps> = ({
  performer,
  onEdit,
  onAutoTag,
  onMerge,
  onDelete,
}) => {
  const [confirmDelete, setConfirmDelete] = useState(false);

  return (
    <div className="performer-sidebar-actions">
      <Button variant="primary" size="sm" className="edit" onClick={onEdit}>
        <FormattedMessage id="actions.edit" />
      </Button>
      <Dropdown className="performer-sidebar-more">
        <Dropdown.Toggle
          variant="secondary"
          size="sm"
          id="performer-more-actions"
        >
          <FormattedMessage id="actions.more" /> ⋯
        </Dropdown.Toggle>
        <Dropdown.Menu align="right" popperConfig={{ strategy: "fixed" }}>
          <Dropdown.Item
            onClick={onAutoTag}
            disabled={performer.ignore_auto_tag}
          >
            <FormattedMessage id="actions.auto_tag" />
          </Dropdown.Item>
          <Dropdown.Item onClick={onMerge}>
            <FormattedMessage id="actions.merge" />…
          </Dropdown.Item>
          <Dropdown.Divider />
          <PerformerImageSearchButton name={performer.name} asDropdownItems />
          <Dropdown.Divider />
          <PerformerSubmitButton performer={performer} asDropdownItem />
          <ShareButton
            shareType={GQL.ShareType.Performer}
            performerId={performer.id}
            label="Share…"
            asDropdownItem
          />
          <Dropdown.Divider />
          <Dropdown.Item
            className="text-danger"
            onClick={() => setConfirmDelete(true)}
          >
            <FormattedMessage id="actions.delete" />
          </Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown>

      {/* the same confirmation DetailsEditNavbar showed before this row replaced it */}
      <Modal show={confirmDelete} onHide={() => setConfirmDelete(false)}>
        <Modal.Body>
          <FormattedMessage
            id="dialogs.delete_confirm"
            values={{ entityName: performer.name }}
          />
        </Modal.Body>
        <Modal.Footer>
          <Button variant="danger" onClick={onDelete}>
            <FormattedMessage id="actions.delete" />
          </Button>
          <Button variant="secondary" onClick={() => setConfirmDelete(false)}>
            <FormattedMessage id="actions.cancel" />
          </Button>
        </Modal.Footer>
      </Modal>
    </div>
  );
};

export default PerformerSidebarActions;
