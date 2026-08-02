import React, { useState } from "react";
import { Dropdown } from "react-bootstrap";
import { Capability, ShareType } from "src/core/generated-graphql";
import { useCapability } from "src/hooks/useCapability";
import { CreateShareModal } from "./CreateShareModal";

interface IProps {
  shareType: ShareType;
  sceneId?: string;
  imageId?: string;
  performerId?: string;
  asDropdownItem?: boolean;
  label?: string;
}

export const ShareButton: React.FC<IProps> = ({
  shareType,
  sceneId,
  imageId,
  performerId,
  asDropdownItem,
  label,
}) => {
  const [show, setShow] = useState(false);
  const { allowed } = useCapability(Capability.ManageShares);

  // Hidden rather than disabled: an account without MANAGE_SHARES cannot create
  // a link at all, so a greyed-out control would only invite the question of
  // how to enable it. Enforcement is server-side regardless.
  if (!allowed) {
    return null;
  }

  const text = label ?? "Share…";

  return (
    <>
      {asDropdownItem ? (
        <Dropdown.Item
          key="share"
          className="bg-secondary text-white"
          onClick={() => setShow(true)}
        >
          {text}
        </Dropdown.Item>
      ) : (
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => setShow(true)}
        >
          {text}
        </button>
      )}
      <CreateShareModal
        show={show}
        onHide={() => setShow(false)}
        shareType={shareType}
        sceneId={sceneId}
        imageId={imageId}
        performerId={performerId}
      />
    </>
  );
};
