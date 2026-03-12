import React, { useRef, useState, useCallback, useEffect } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import Cropper from "cropperjs";
import "cropperjs/dist/cropper.min.css";
import { usePerformerUpdate } from "src/core/StashService";
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
  const [updatePerformer] = usePerformerUpdate();
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
      initialAspectRatio: 2 / 3,
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

    const dataUrl = cropperRef.current.getCroppedCanvas().toDataURL();
    destroyCropper();
    setCropping(false);

    try {
      await updatePerformer({
        variables: {
          input: {
            id: performerId,
            image: dataUrl,
          },
        },
      });

      // Force reload the performer image to show the updated crop
      const image = getPerformerImage();
      if (image) {
        const url = new URL(image.src, window.location.origin);
        url.searchParams.set("t", Date.now().toString());
        image.src = url.toString();
      }

      Toast.success("Image cropped successfully");
    } catch (e) {
      Toast.error(e);
    }
  }, [destroyCropper, setCropping, updatePerformer, performerId, getPerformerImage, Toast]);

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
