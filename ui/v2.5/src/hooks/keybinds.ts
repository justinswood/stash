import Mousetrap from "mousetrap";
import { useEffect, useRef } from "react";
import { defaultRatingSystemType, RatingSystemType } from "src/utils/rating";

export function useRatingKeybinds(
  isVisible: boolean,
  ratingSystem: RatingSystemType | undefined,
  setRating: (v: number) => void
) {
  const firstChar = useRef<string | undefined>(undefined);

  const starRatingShortcuts: { [char: string]: number } = {
    "0": NaN,
    "1": 20,
    "2": 40,
    "3": 60,
    "4": 80,
    "5": 100,
  };

  function handleStarRatingKeybinds() {
    for (const key in starRatingShortcuts) {
      Mousetrap.bind(key, () => setRating(starRatingShortcuts[key]));
    }

    setTimeout(() => {
      for (const key in starRatingShortcuts) {
        Mousetrap.unbind(key);
      }
    }, 1000);
  }

  function handleDecimalKeybinds() {
    Mousetrap.bind("`", () => {
      setRating(NaN);
    });

    for (let i = 0; i <= 9; ++i) {
      Mousetrap.bind(i.toString(), () => {
        if (firstChar.current !== undefined) {
          let combined = parseInt(firstChar.current + i.toString());
          if (combined === 0) {
            combined = 100;
          }

          setRating(combined);
          firstChar.current = undefined;
        } else {
          firstChar.current = i.toString();
        }
      });
    }

    setTimeout(() => {
      firstChar.current = undefined;

      Mousetrap.unbind("`");
      for (let i = 0; i <= 9; ++i) {
        Mousetrap.unbind(i.toString());
      }
    }, 1000);
  }

  useEffect(() => {
    if (!isVisible) return;

    Mousetrap.bind("r", () => {
      // numeric keypresses get caught by jwplayer, so blur the element
      // if the rating sequence is started
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }

      // Fall back to the configured default, not to stars. All six callers
      // pass configuration?.ui.ratingSystemOptions?.type, which is undefined
      // for any account that has never customised it — so `!ratingSystem`
      // meant those accounts rendered decimal ratings but got star keybinds.
      const effectiveSystem = ratingSystem ?? defaultRatingSystemType;
      if (effectiveSystem === RatingSystemType.Stars) {
        return handleStarRatingKeybinds();
      } else {
        return handleDecimalKeybinds();
      }
    });

    return () => {
      Mousetrap.unbind("r");
    };
  });
}
