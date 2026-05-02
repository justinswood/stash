import React, { useState } from "react";
import { Dropdown } from "react-bootstrap";
import { ShareType } from "src/core/generated-graphql";
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
