import React, { useRef, useState, useCallback, useEffect } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import Cropper from "cropperjs";
import "cropperjs/dist/cropper.min.css";
import { usePerformerImageCrop } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";

interface IPerformerImageCropperProps {
  performerId: string;
  imageUrl: string;
  onCroppingChange?: (cropping: boolean) => void;
}

export const PerformerImageCropper: React.FC<IPerformerImageCropperProps> = ({
  performerId,
  imageUrl,
  onCroppingChange,
}) => {
  const Toast = useToast();
  const [cropPerformerImage] = usePerformerImageCrop();
  const [cropping, setCroppingState] = useState(false);
  const [cropReady, setCropReady] = useState(false);
  const [cropInfo, setCropInfo] = useState("");
  const cropperRef = useRef<Cropper | null>(null);

  const setCropping = useCallback(
    (value: boolean) => {
      setCroppingState(value);
      onCroppingChange?.(value);
    },
    [onCroppingChange]
  );

  // Find the performer image element in the DOM
  const getPerformerImage = useCallback((): HTMLImageElement | null => {
    return document.querySelector(
      ".detail-header-image img.performer"
    ) as HTMLImageElement | null;
  }, []);

  // Cleanup cropper instance
  const destroyCropper = useCallback(() => {
    if (cropperRef.current) {
      cropperRef.current.destroy();
      cropperRef.current = null;
    }
    setCropReady(false);
    setCropInfo("");
  }, []);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (cropperRef.current) {
        cropperRef.current.destroy();
        cropperRef.current = null;
      }
    };
  }, []);

  const handleCropStart = useCallback(() => {
    const image = getPerformerImage();
    if (!image) return;

    setCropping(true);

    cropperRef.current = new Cropper(image, {
      viewMode: 1,
      // Locked, not initialAspectRatio. Performer cards render the image in a
      // 2:3 frame with object-fit: cover (Performers/styles.scss), so a crop of
      // any other shape gets silently re-cropped there — a 1000x700 selection
      // shows only 47% of its width on a card. Constraining the selection keeps
      // what you pick and what the card shows the same thing.
      aspectRatio: 2 / 3,
      movable: false,
      rotatable: false,
      scalable: false,
      zoomable: false,
      zoomOnTouch: false,
      zoomOnWheel: false,
      ready() {
        setCropReady(true);
      },
      crop(e) {
        setCropInfo(
          `X: ${Math.round(e.detail.x)}, Y: ${Math.round(e.detail.y)}, ` +
            `${Math.round(e.detail.width)}px × ${Math.round(e.detail.height)}px`
        );
      },
    });
  }, [getPerformerImage, setCropping]);

  const handleCropAccept = useCallback(async () => {
    if (!cropperRef.current) return;

    // Only the selected rectangle is sent; the server cuts it out of the stored
    // image. Cropper is used purely to pick the region.
    //
    // The pixels are deliberately NOT produced here. Doing that requires
    // reading them back out of a canvas, and browsers with canvas
    // anti-fingerprinting protection (Gecko with canvas protection enabled, as
    // Zen ships by default) return blank data for any canvas past a small size
    // threshold — silently, with no error. That previously wrote a fully
    // transparent image over the performer's photo, which showed up as black.
    const data = cropperRef.current.getData(true);
    const input = {
      performer_id: performerId,
      x: Math.max(0, Math.round(data.x)),
      y: Math.max(0, Math.round(data.y)),
      width: Math.round(data.width),
      height: Math.round(data.height),
    };

    if (input.width <= 0 || input.height <= 0) {
      Toast.error("Select an area to crop first.");
      return;
    }

    destroyCropper();
    setCropping(false);

    try {
      // The mutation bumps the performer's updated_at, which changes the "t"
      // param in image_path, so the re-rendered <img> requests a URL the browser
      // has not cached. Don't rewrite image.src by hand: React re-renders from
      // image_path straight afterwards and would put the cached URL back.
      await cropPerformerImage({ variables: { input } });

      Toast.success("Image cropped successfully");
    } catch (e) {
      Toast.error(e);
    }
  }, [destroyCropper, setCropping, cropPerformerImage, performerId, Toast]);

  const handleCropCancel = useCallback(() => {
    destroyCropper();
    setCropping(false);
  }, [destroyCropper, setCropping]);

  return (
    <div className="performer-image-cropper">
      {!cropping ? (
        <Button
          variant="secondary"
          size="sm"
          onClick={handleCropStart}
          className="crop-start-btn"
        >
          <FormattedMessage id="actions.crop_image" />
        </Button>
      ) : (
        <div className="crop-controls">
          <Button
            variant="success"
            size="sm"
            onClick={handleCropAccept}
            disabled={!cropReady}
            className="mr-2"
          >
            <FormattedMessage id="actions.confirm" />
          </Button>
          <Button variant="danger" size="sm" onClick={handleCropCancel}>
            <FormattedMessage id="actions.cancel" />
          </Button>
          {cropInfo && <span className="crop-info">{cropInfo}</span>}
        </div>
      )}
    </div>
  );
};
