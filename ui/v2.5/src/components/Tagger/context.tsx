import React, { useState, useEffect, useRef, useMemo } from "react";
import { initialConfig, ITaggerConfig } from "src/components/Tagger/constants";
import * as GQL from "src/core/generated-graphql";
import {
  queryFindPerformer,
  queryFindStudio,
  queryScrapeScene,
  queryScrapeSceneQuery,
  queryScrapeSceneQueryFragment,
  stashBoxSceneBatchQuery,
  useListSceneScrapers,
  usePerformerCreate,
  usePerformerUpdate,
  useSceneUpdate,
  useStudioCreate,
  useStudioUpdate,
  useTagCreate,
  useTagUpdate,
} from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import { useConfigurationContext } from "src/hooks/Config";
import { ITaggerSource, SCRAPER_PREFIX, STASH_BOX_PREFIX } from "./constants";
import { errorToString } from "src/utils";
import { mergeStudioStashIDs, prepareQueryString, parsePath } from "./utils";
import { objectPath } from "src/core/files";
import { useTaggerConfig } from "./config";

const BATCH_SEARCH_DELAY = 200;

export interface ITaggerContextState {
  config: ITaggerConfig;
  setConfig: (c: ITaggerConfig) => void;
  loading: boolean;
  loadingMulti?: boolean;
  multiError?: string;
  sources: ITaggerSource[];
  currentSource?: ITaggerSource;
  searchResults: Record<string, ISceneQueryResult>;
  setCurrentSource: (src?: ITaggerSource) => void;
  doSceneQuery: (sceneID: string, searchStr: string) => Promise<void>;
  doSceneCascadeSearch: (sceneID: string, searchStr: string) => Promise<void>;
  doSceneFragmentScrape: (sceneID: string) => Promise<void>;
  doMultiSceneFragmentScrape: (sceneIDs: string[]) => Promise<void>;
  stopMultiScrape: () => void;
  createNewTag: (
    tag: GQL.ScrapedTag,
    toCreate: GQL.TagCreateInput
  ) => Promise<string | undefined>;
  createNewPerformer: (
    performer: GQL.ScrapedPerformer,
    toCreate: GQL.PerformerCreateInput
  ) => Promise<string | undefined>;
  linkPerformer: (
    performer: GQL.ScrapedPerformer,
    performerID: string,
    endpoint?: string
  ) => Promise<void>;
  createNewStudio: (
    studio: GQL.ScrapedStudio,
    toCreate: GQL.StudioCreateInput
  ) => Promise<string | undefined>;
  updateStudio: (
    studio: GQL.StudioUpdateInput,
    endpoint?: string
  ) => Promise<void>;
  linkStudio: (
    studio: GQL.ScrapedStudio,
    studioID: string,
    endpoint?: string
  ) => Promise<void>;
  updateTag: (
    tag: GQL.ScrapedTag,
    updateInput: GQL.TagUpdateInput
  ) => Promise<void>;
  resolveScene: (
    sceneID: string,
    index: number,
    scene: IScrapedScene
  ) => Promise<void>;
  submitFingerprints: () => Promise<void>;
  pendingFingerprints: string[];
  saveScene: (
    sceneCreateInput: GQL.SceneUpdateInput,
    queueFingerprint: boolean,
    endpoint?: string
  ) => Promise<void>;
  registerSaveCallback: (
    sceneID: string,
    fn: () => Promise<void>
  ) => void;
  unregisterSaveCallback: (sceneID: string) => void;
  doBatchSave: () => Promise<void>;
  stopBatchSave: () => void;
  loadingBatchSave: boolean;
  batchSaveProgress: number;
  batchSaveCount: number;
  doBatchSearch: (
    scenes: GQL.SlimSceneDataFragment[],
    cascade?: boolean
  ) => Promise<void>;
  stopBatchSearch: () => void;
  loadingBatchSearch: boolean;
  batchSearchProgress: number;
}

const dummyFn = () => {
  return Promise.resolve();
};
const dummyValFn = () => {
  return Promise.resolve(undefined);
};

export const TaggerStateContext = React.createContext<ITaggerContextState>({
  config: initialConfig,
  setConfig: () => {},
  loading: false,
  sources: [],
  searchResults: {},
  setCurrentSource: () => {},
  doSceneQuery: dummyFn,
  doSceneCascadeSearch: dummyFn,
  doSceneFragmentScrape: dummyFn,
  doMultiSceneFragmentScrape: dummyFn,
  stopMultiScrape: () => {},
  createNewTag: dummyValFn,
  createNewPerformer: dummyValFn,
  linkPerformer: dummyFn,
  createNewStudio: dummyValFn,
  updateStudio: dummyFn,
  linkStudio: dummyFn,
  updateTag: dummyFn,
  resolveScene: dummyFn,
  submitFingerprints: dummyFn,
  pendingFingerprints: [],
  saveScene: dummyFn,
  registerSaveCallback: () => {},
  unregisterSaveCallback: () => {},
  doBatchSave: dummyFn,
  stopBatchSave: () => {},
  loadingBatchSave: false,
  batchSaveProgress: 0,
  batchSaveCount: 0,
  doBatchSearch: dummyFn,
  stopBatchSearch: () => {},
  loadingBatchSearch: false,
  batchSearchProgress: 0,
});

export type IScrapedScene = GQL.ScrapedScene & {
  resolved?: boolean;
  // the source that produced this result. Results from every source are merged
  // into one ranked list, so attribution has to travel with the result itself
  // rather than with the result set.
  source?: ITaggerSource;
};

export interface ISceneQueryResult {
  results?: IScrapedScene[];
  error?: string;
  // set when every source was searched and none of them returned anything
  cascadeExhausted?: boolean;
  // the source these results were scraped from, for a single-source search.
  // A multi-source search leaves this unset and tags each result instead.
  source?: ITaggerSource;
}

export const TaggerContext: React.FC = ({ children }) => {
  const [loading, setLoading] = useState(false);
  const [loadingMulti, setLoadingMulti] = useState(false);
  const [sources, setSources] = useState<ITaggerSource[]>([]);
  const [currentSource, setCurrentSource] = useState<ITaggerSource>();
  const [multiError, setMultiError] = useState<string | undefined>();
  const [searchResults, setSearchResults] = useState<
    Record<string, ISceneQueryResult>
  >({});

  const stopping = useRef(false);
  const stoppingCascade = useRef(false);

  const { configuration: stashConfig } = useConfigurationContext();
  const { config, setConfig } = useTaggerConfig();

  const Scrapers = useListSceneScrapers();

  const Toast = useToast();
  const [createTag] = useTagCreate();
  const [createPerformer] = usePerformerCreate();
  const [updatePerformer] = usePerformerUpdate();
  const [createStudio] = useStudioCreate();
  const [updateStudio] = useStudioUpdate();
  const [updateScene] = useSceneUpdate();
  const [updateTag] = useTagUpdate();

  useEffect(() => {
    if (!stashConfig || !Scrapers.data) {
      return;
    }

    const { stashBoxes } = stashConfig.general;
    const scrapers = Scrapers.data.listScrapers;

    const stashboxSources: ITaggerSource[] = stashBoxes.map((s, i) => ({
      id: `${STASH_BOX_PREFIX}${s.endpoint}`,
      sourceInput: {
        stash_box_endpoint: s.endpoint,
      },
      displayName: `stash-box: ${s.name || `#${i + 1}`}`,
      supportSceneFragment: true,
      supportSceneQuery: true,
    }));

    // filter scraper sources such that only those that can query scrape or
    // scrape via fragment are added
    const scraperSources: ITaggerSource[] = scrapers
      .filter((s) =>
        s.scene?.supported_scrapes.some(
          (t) => t === GQL.ScrapeType.Name || t === GQL.ScrapeType.Fragment
        )
      )
      .map((s) => ({
        id: `${SCRAPER_PREFIX}${s.id}`,
        sourceInput: {
          scraper_id: s.id,
        },
        displayName: s.name,
        supportSceneQuery: s.scene?.supported_scrapes.includes(
          GQL.ScrapeType.Name
        ),
        supportSceneFragment: s.scene?.supported_scrapes.includes(
          GQL.ScrapeType.Fragment
        ),
      }));

    setSources(stashboxSources.concat(scraperSources));
  }, [Scrapers.data, stashConfig]);

  // set the current source on load
  useEffect(() => {
    if (!sources.length || currentSource) {
      return;
    }
    // First, see if we have a saved endpoint.
    if (config.selectedEndpoint) {
      let source = sources.find(
        (s) => s.sourceInput.stash_box_endpoint == config.selectedEndpoint
      );
      if (source) {
        setCurrentSource(source);
        return;
      }
    }
    // Otherwise, just use the first source.
    setCurrentSource(sources[0]);
  }, [sources, currentSource, config]);

  // clear the search results when the source changes
  useEffect(() => {
    setSearchResults({});
  }, [currentSource]);

  // keep selected endpoint in config in sync with current source
  useEffect(() => {
    const selectedEndpoint = currentSource?.sourceInput.stash_box_endpoint;
    if (selectedEndpoint && selectedEndpoint !== config.selectedEndpoint) {
      setConfig({
        ...config,
        selectedEndpoint,
      });
    }
  }, [currentSource, config, setConfig]);

  function getPendingFingerprints() {
    const endpoint = currentSource?.sourceInput.stash_box_endpoint;
    if (!config || !endpoint) return [];

    return config.fingerprintQueue[endpoint] ?? [];
  }

  function clearSubmissionQueue() {
    const endpoint = currentSource?.sourceInput.stash_box_endpoint;
    if (!config || !endpoint) return;

    setConfig({
      ...config,
      fingerprintQueue: {
        ...config.fingerprintQueue,
        [endpoint]: [],
      },
    });
  }

  const [submitFingerprintsMutation] =
    GQL.useSubmitStashBoxFingerprintsMutation();

  async function submitFingerprints() {
    const endpoint = currentSource?.sourceInput.stash_box_endpoint;

    if (!config || !endpoint) return;

    try {
      setLoading(true);
      await submitFingerprintsMutation({
        variables: {
          input: {
            stash_box_endpoint: endpoint,
            scene_ids: config.fingerprintQueue[endpoint],
          },
        },
      });

      clearSubmissionQueue();
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
    }
  }

  // endpoint defaults to the selected source, but a result found by a cascade
  // must submit its fingerprint to the stash-box that actually matched it
  function queueFingerprintSubmission(sceneId: string, endpoint?: string) {
    const ep = endpoint ?? currentSource?.sourceInput.stash_box_endpoint;
    if (!config || !ep) return;

    setConfig({
      ...config,
      fingerprintQueue: {
        ...config.fingerprintQueue,
        [ep]: [...(config.fingerprintQueue[ep] ?? []), sceneId],
      },
    });
  }

  function clearSearchResults(sceneID: string) {
    setSearchResults((current) => {
      const newSearchResults = { ...current };
      delete newSearchResults[sceneID];
      return newSearchResults;
    });
  }

  // Scrapes by name against one source and returns the outcome without
  // touching state, so a multi-source search can accumulate across sources
  // rather than each call clobbering the last.
  async function scrapeByQuery(
    searchVal: string,
    src: ITaggerSource
  ): Promise<ISceneQueryResult> {
    try {
      const results = await queryScrapeSceneQuery(src.sourceInput, searchVal);
      // scenes are already resolved if they come from stash-box
      const resolved = src.sourceInput.stash_box_endpoint !== undefined;

      if (results.error) {
        return { error: results.error.message, source: src };
      }
      if (results.errors) {
        return { error: results.errors.toString(), source: src };
      }
      return {
        results: results.data.scrapeSingleScene.map((r) => ({
          ...r,
          resolved,
          source: src,
        })),
        source: src,
      };
    } catch (err: unknown) {
      return { error: errorToString(err), source: src };
    }
  }

  async function sceneQuerySearch(
    sceneID: string,
    searchVal: string,
    source?: ITaggerSource
  ) {
    const src = source ?? currentSource;
    if (!src) {
      return;
    }

    clearSearchResults(sceneID);

    const newResult = await scrapeByQuery(searchVal, src);

    setSearchResults((current) => {
      return { ...current, [sceneID]: newResult };
    });

    return newResult;
  }

  async function doSceneQuery(sceneID: string, searchVal: string) {
    if (!currentSource) {
      return;
    }

    try {
      setLoading(true);
      await sceneQuerySearch(sceneID, searchVal);
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
    }
  }

  // The order sources are searched: the selected source first, since that is
  // the user's stated preference, then the rest in dropdown order.
  const cascadeSources = useMemo(() => {
    if (!currentSource) return sources;
    return [currentSource, ...sources.filter((s) => s.id !== currentSource.id)];
  }, [sources, currentSource]);

  // Searches EVERY source and merges what they return into one list, each
  // result carrying the source that produced it.
  //
  // This deliberately does not stop at the first source that returns
  // something, the way the identify task does. Identify matches on
  // fingerprints, where a hit is exact and "found something" means "found the
  // right thing". A name query is fuzzy: stash-box returns up to ten loose
  // matches for any string at all - measured, it returns five results for pure
  // gibberish - so "the first source with a result" would always be the first
  // source tried, and no other source would ever be reached.
  //
  // Merging instead lets compareScenesForSort rank every candidate against the
  // local file's own phash and duration, which is source-agnostic, so a real
  // match from any source floats above another source's fuzzy noise.
  async function searchAllSources(sceneID: string, searchVal: string) {
    clearSearchResults(sceneID);

    const merged: IScrapedScene[] = [];
    const errors: string[] = [];

    for (const source of cascadeSources) {
      if (stoppingCascade.current) break;

      let result: ISceneQueryResult;
      if (source.supportSceneQuery) {
        result = await scrapeByQuery(searchVal, source);
      } else if (source.supportSceneFragment) {
        result = await scrapeByFragment(sceneID, source);
      } else {
        continue;
      }

      if (result.error) {
        errors.push(`${source.displayName}: ${result.error}`);
      } else if (result.results?.length) {
        merged.push(...result.results);
      }

      // publish as we go so results appear source by source rather than after
      // every source has been waited on
      const soFar: ISceneQueryResult = { results: [...merged] };
      setSearchResults((current) => {
        return { ...current, [sceneID]: soFar };
      });
    }

    // A source that failed is not the same as a source that found nothing, so
    // report the failures rather than letting "no results" imply every source
    // answered. When some source did return candidates the errors are dropped:
    // the user has results to work with and one dead scraper is just noise.
    const final: ISceneQueryResult =
      merged.length === 0 && errors.length > 0
        ? { error: errors.join("; ") }
        : { results: merged, cascadeExhausted: merged.length === 0 };

    setSearchResults((current) => {
      return { ...current, [sceneID]: final };
    });

    return final;
  }

  async function doSceneCascadeSearch(sceneID: string, searchVal: string) {
    if (!cascadeSources.length) {
      return;
    }

    try {
      stoppingCascade.current = false;
      setLoading(true);
      await searchAllSources(sceneID, searchVal);
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
    }
  }

  // fragment-scrape counterpart of scrapeByQuery - no state, returns outcome
  async function scrapeByFragment(
    sceneID: string,
    src: ITaggerSource
  ): Promise<ISceneQueryResult> {
    try {
      const results = await queryScrapeScene(src.sourceInput, sceneID);

      if (results.error) {
        return { error: results.error.message, source: src };
      }
      if (results.errors) {
        return { error: results.errors.toString(), source: src };
      }
      return {
        results: results.data.scrapeSingleScene.map((r) => ({
          ...r,
          // scenes are already resolved if they are scraped via fragment
          resolved: true,
          source: src,
        })),
        source: src,
      };
    } catch (err: unknown) {
      return { error: errorToString(err), source: src };
    }
  }

  async function sceneFragmentScrape(sceneID: string, source?: ITaggerSource) {
    const src = source ?? currentSource;
    if (!src) {
      return;
    }

    clearSearchResults(sceneID);

    const newResult = await scrapeByFragment(sceneID, src);

    setSearchResults((current) => {
      return { ...current, [sceneID]: newResult };
    });

    return newResult;
  }

  async function doSceneFragmentScrape(sceneID: string) {
    if (!currentSource) {
      return;
    }

    clearSearchResults(sceneID);

    try {
      setLoading(true);
      await sceneFragmentScrape(sceneID);
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
    }
  }

  async function doMultiSceneFragmentScrape(sceneIDs: string[]) {
    if (!currentSource) {
      return;
    }

    setSearchResults({});

    try {
      stopping.current = false;
      setLoading(true);
      setMultiError(undefined);

      const stashBoxEndpoint =
        currentSource.sourceInput.stash_box_endpoint ?? undefined;

      // if current source is stash-box, we can use the multi-scene
      // interface
      if (stashBoxEndpoint !== undefined) {
        const results = await stashBoxSceneBatchQuery(
          sceneIDs,
          stashBoxEndpoint
        );

        if (results.error) {
          setMultiError(results.error.message);
        } else if (results.errors) {
          setMultiError(results.errors.toString());
        } else {
          const newSearchResults = { ...searchResults };
          sceneIDs.forEach((sceneID, index) => {
            const newResults = results.data.scrapeMultiScenes[index].map(
              (r) => ({
                ...r,
                resolved: true,
                source: currentSource,
              })
            );

            newSearchResults[sceneID] = {
              results: newResults,
              source: currentSource,
            };
          });

          setSearchResults(newSearchResults);
        }
      } else {
        setLoadingMulti(true);

        // do singular calls
        await sceneIDs.reduce(async (promise, id) => {
          await promise;
          if (!stopping.current) {
            await sceneFragmentScrape(id);
          }
        }, Promise.resolve());
      }
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
      setLoadingMulti(false);
    }
  }

  function stopMultiScrape() {
    stopping.current = true;
  }

  // Batch save
  const saveCallbacks = useRef<Map<string, () => Promise<void>>>(new Map());
  const stoppingBatchSave = useRef(false);
  const [loadingBatchSave, setLoadingBatchSave] = useState(false);
  const [batchSaveProgress, setBatchSaveProgress] = useState(0);
  const [batchSaveCount, setBatchSaveCount] = useState(0);

  function registerSaveCallback(sceneID: string, fn: () => Promise<void>) {
    saveCallbacks.current.set(sceneID, fn);
    setBatchSaveCount(saveCallbacks.current.size);
  }

  function unregisterSaveCallback(sceneID: string) {
    saveCallbacks.current.delete(sceneID);
    setBatchSaveCount(saveCallbacks.current.size);
  }

  async function doBatchSave() {
    const callbacks = Array.from(saveCallbacks.current.entries());
    if (callbacks.length === 0) return;

    stoppingBatchSave.current = false;
    setLoadingBatchSave(true);
    setBatchSaveProgress(0);

    let completed = 0;
    for (const [, saveFn] of callbacks) {
      if (stoppingBatchSave.current) break;
      try {
        await saveFn();
      } catch (err) {
        Toast.error(err);
      }
      completed++;
      setBatchSaveProgress((completed / callbacks.length) * 100);
    }

    setLoadingBatchSave(false);
    setBatchSaveProgress(0);
  }

  function stopBatchSave() {
    stoppingBatchSave.current = true;
  }

  // Batch search
  const stoppingBatchSearch = useRef(false);
  const [loadingBatchSearch, setLoadingBatchSearch] = useState(false);
  const [batchSearchProgress, setBatchSearchProgress] = useState(0);

  // when cascade is set, each scene is searched against every source in turn
  // until one matches, rather than against the selected source alone
  async function doBatchSearch(
    scenes: GQL.SlimSceneDataFragment[],
    cascade?: boolean
  ) {
    if (scenes.length === 0) return;
    if (cascade ? !cascadeSources.length : !currentSource) return;

    stoppingBatchSearch.current = false;
    stoppingCascade.current = false;
    setLoadingBatchSearch(true);
    setBatchSearchProgress(0);

    let completed = 0;
    for (const scene of scenes) {
      if (stoppingBatchSearch.current) break;

      const { paths, file: basename } = parsePath(objectPath(scene));
      const queryString = prepareQueryString(
        scene,
        paths,
        basename,
        config.mode,
        config.blacklist
      );

      try {
        if (cascade) {
          await searchAllSources(scene.id, queryString);
        } else {
          await sceneQuerySearch(scene.id, queryString);
        }
      } catch (err) {
        Toast.error(err);
      }

      completed++;
      setBatchSearchProgress((completed / scenes.length) * 100);

      // delay between searches to avoid overwhelming the server
      if (!stoppingBatchSearch.current && completed < scenes.length) {
        await new Promise((r) => setTimeout(r, BATCH_SEARCH_DELAY));
      }
    }

    setLoadingBatchSearch(false);
    setBatchSearchProgress(0);
  }

  function stopBatchSearch() {
    stoppingBatchSearch.current = true;
    stoppingCascade.current = true;
  }

  async function resolveScene(
    sceneID: string,
    index: number,
    scene: IScrapedScene
  ) {
    // Resolve against the source that produced this particular result. A
    // multi-source search merges results from several sources into one list,
    // so the result's own source is authoritative; the set-level source only
    // exists for a single-source search.
    const src = scene.source ?? searchResults[sceneID]?.source ?? currentSource;
    if (!src || scene.resolved || !searchResults[sceneID].results) {
      return Promise.resolve();
    }

    try {
      const sceneInput: GQL.ScrapedSceneInput = {
        date: scene.date,
        details: scene.details,
        remote_site_id: scene.remote_site_id,
        title: scene.title,
        urls: scene.urls,
      };

      const result = await queryScrapeSceneQueryFragment(
        src.sourceInput,
        sceneInput
      );

      if (result.data.scrapeSingleScene.length) {
        const resolvedScene = result.data.scrapeSingleScene[0];

        // set the scene in the results and mark as resolved. The resolved
        // payload comes back from the API without a source, so carry the
        // original result's source across or the save path loses its
        // attribution.
        const newResult = [...searchResults[sceneID].results!];
        newResult[index] = { ...resolvedScene, resolved: true, source: src };
        setSearchResults({
          ...searchResults,
          [sceneID]: { ...searchResults[sceneID], results: newResult },
        });
      }
    } catch (err) {
      Toast.error(err);

      const newResult = [...searchResults[sceneID].results!];
      newResult[index] = { ...newResult[index], resolved: true };
      setSearchResults({
        ...searchResults,
        [sceneID]: { ...searchResults[sceneID], results: newResult },
      });
    }
  }

  async function saveScene(
    sceneCreateInput: GQL.SceneUpdateInput,
    queueFingerprint: boolean,
    endpoint?: string
  ) {
    try {
      await updateScene({
        variables: {
          input: {
            ...sceneCreateInput,
            // only set organized if it is enabled in the config
            organized: config?.markSceneAsOrganizedOnSave || undefined,
          },
        },
      });

      if (queueFingerprint) {
        queueFingerprintSubmission(sceneCreateInput.id, endpoint);
      }
      clearSearchResults(sceneCreateInput.id);
    } catch (err) {
      Toast.error(err);
    } finally {
      setLoading(false);
    }
  }

  function mapResults(fn: (r: IScrapedScene) => IScrapedScene) {
    const newSearchResults = { ...searchResults };

    Object.keys(newSearchResults).forEach((k) => {
      const searchResult = searchResults[k];
      if (!searchResult.results) {
        return;
      }

      newSearchResults[k].results = searchResult.results.map(fn);
    });

    return newSearchResults;
  }

  async function createNewTag(
    tag: GQL.ScrapedTag,
    toCreate: GQL.TagCreateInput
  ) {
    try {
      const result = await createTag({
        variables: {
          input: toCreate,
        },
      });

      const tagID = result.data?.tagCreate?.id;
      if (tagID === undefined) return undefined;

      const newSearchResults = mapResults((r) => {
        if (!r.tags) {
          return r;
        }

        return {
          ...r,
          tags: r.tags.map((t) => {
            if (t.name === tag.name) {
              return {
                ...t,
                stored_id: tagID,
              };
            }

            return t;
          }),
        };
      });

      setSearchResults(newSearchResults);

      Toast.success(
        <span>
          Created tag: <b>{toCreate.name}</b>
        </span>
      );

      return tagID;
    } catch (e) {
      Toast.error(e);
    }
  }

  async function createNewPerformer(
    performer: GQL.ScrapedPerformer,
    toCreate: GQL.PerformerCreateInput
  ) {
    try {
      const result = await createPerformer({
        variables: {
          input: toCreate,
        },
      });

      const performerID = result.data?.performerCreate?.id;
      if (performerID === undefined) return undefined;

      const newSearchResults = mapResults((r) => {
        if (!r.performers) {
          return r;
        }

        return {
          ...r,
          performers: r.performers.map((p) => {
            // Match by remote_site_id if available, otherwise fall back to name
            const matches = performer.remote_site_id
              ? p.remote_site_id === performer.remote_site_id
              : p.name === performer.name;

            if (matches) {
              return {
                ...p,
                stored_id: performerID,
              };
            }

            return p;
          }),
        };
      });

      setSearchResults(newSearchResults);

      Toast.success(
        <span>
          Created performer: <b>{toCreate.name}</b>
        </span>
      );

      return performerID;
    } catch (e) {
      Toast.error(e);
    }
  }

  async function linkPerformer(
    performer: GQL.ScrapedPerformer,
    performerID: string,
    endpoint?: string
  ) {
    const ep = endpoint ?? currentSource?.sourceInput.stash_box_endpoint;
    if (!performer.remote_site_id || !ep) return;

    try {
      const queryResult = await queryFindPerformer(performerID);
      if (queryResult.data.findPerformer) {
        const target = queryResult.data.findPerformer;

        const stashIDs: GQL.StashIdInput[] = target.stash_ids.map((e) => {
          return {
            endpoint: e.endpoint,
            stash_id: e.stash_id,
            updated_at: e.updated_at,
          };
        });

        stashIDs.push({
          stash_id: performer.remote_site_id,
          endpoint: ep,
          updated_at: new Date().toISOString(),
        });

        await updatePerformer({
          variables: {
            input: {
              id: performerID,
              stash_ids: stashIDs,
            },
          },
        });

        const newSearchResults = mapResults((r) => {
          if (!r.performers) {
            return r;
          }

          return {
            ...r,
            performers: r.performers.map((p) => {
              if (p.remote_site_id === performer.remote_site_id) {
                return {
                  ...p,
                  stored_id: performerID,
                };
              }

              return p;
            }),
          };
        });

        setSearchResults(newSearchResults);

        Toast.success(<span>Added stash-id to performer</span>);
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  async function createNewStudio(
    studio: GQL.ScrapedStudio,
    toCreate: GQL.StudioCreateInput
  ) {
    try {
      const result = await createStudio({
        variables: {
          input: toCreate,
        },
      });

      const studioID = result.data?.studioCreate?.id;
      if (studioID === undefined) return undefined;

      const newSearchResults = mapResults((r) => {
        if (!r.studio) {
          return r;
        }

        let resultStudio = r.studio;
        if (resultStudio.name === studio.name) {
          resultStudio = {
            ...resultStudio,
            stored_id: studioID,
          };
        }

        // #5821 - set the stored_id of the parent studio if it matches too
        if (resultStudio.parent?.name === studio.name) {
          resultStudio = {
            ...resultStudio,
            parent: {
              ...resultStudio.parent,
              stored_id: studioID,
            },
          };
        }

        return {
          ...r,
          studio: resultStudio,
        };
      });

      setSearchResults(newSearchResults);

      Toast.success(
        <span>
          Created studio: <b>{toCreate.name}</b>
        </span>
      );

      return studioID;
    } catch (e) {
      Toast.error(e);
    }
  }

  async function updateExistingStudio(
    input: GQL.StudioUpdateInput,
    endpoint?: string
  ) {
    try {
      const inputCopy = { ...input };
      inputCopy.stash_ids = await mergeStudioStashIDs(
        input.id,
        input.stash_ids ?? []
      );
      const result = await updateStudio({
        variables: {
          input: input,
        },
      });

      const studioID = result.data?.studioUpdate?.id;

      const stashID = input.stash_ids?.find((e) => {
        return (
          e.endpoint ===
          (endpoint ?? currentSource?.sourceInput.stash_box_endpoint)
        );
      })?.stash_id;

      if (stashID) {
        const newSearchResults = mapResults((r) => {
          if (!r.studio) {
            return r;
          }

          return {
            ...r,
            studio:
              r.remote_site_id === stashID
                ? {
                    ...r.studio,
                    stored_id: studioID,
                  }
                : r.studio,
          };
        });

        setSearchResults(newSearchResults);
      }

      Toast.success(
        <span>
          Created studio: <b>{input.name}</b>
        </span>
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  async function linkStudio(
    studio: GQL.ScrapedStudio,
    studioID: string,
    endpoint?: string
  ) {
    const ep = endpoint ?? currentSource?.sourceInput.stash_box_endpoint;
    if (!studio.remote_site_id || !ep) return;

    try {
      const queryResult = await queryFindStudio(studioID);
      if (queryResult.data.findStudio) {
        const target = queryResult.data.findStudio;

        const stashIDs: GQL.StashIdInput[] = target.stash_ids.map((e) => {
          return {
            endpoint: e.endpoint,
            stash_id: e.stash_id,
            updated_at: e.updated_at,
          };
        });

        stashIDs.push({
          stash_id: studio.remote_site_id,
          endpoint: ep,
          updated_at: new Date().toISOString(),
        });

        await updateStudio({
          variables: {
            input: {
              id: studioID,
              stash_ids: stashIDs,
            },
          },
        });

        const newSearchResults = mapResults((r) => {
          if (!r.studio) {
            return r;
          }

          return {
            ...r,
            studio:
              r.studio.remote_site_id === studio.remote_site_id
                ? {
                    ...r.studio,
                    stored_id: studioID,
                  }
                : r.studio,
          };
        });

        setSearchResults(newSearchResults);

        Toast.success(<span>Added stash-id to studio</span>);
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  async function updateExistingTag(
    tag: GQL.ScrapedTag,
    updateInput: GQL.TagUpdateInput
  ) {
    const hasRemoteID = !!tag.remote_site_id;

    try {
      await updateTag({
        variables: {
          input: updateInput,
        },
      });

      const newSearchResults = mapResults((r) => {
        if (!r.tags) {
          return r;
        }

        return {
          ...r,
          tags: r.tags.map((t) => {
            if (
              (hasRemoteID && t.remote_site_id === tag.remote_site_id) ||
              (!hasRemoteID && t.name === tag.name)
            ) {
              return {
                ...t,
                stored_id: updateInput.id,
              };
            }

            return t;
          }),
        };
      });

      setSearchResults(newSearchResults);

      Toast.success(<span>Updated tag</span>);
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <TaggerStateContext.Provider
      value={{
        config: config ?? initialConfig,
        setConfig,
        loading: loading || loadingMulti,
        loadingMulti,
        multiError,
        sources,
        currentSource,
        searchResults,
        setCurrentSource: (src) => {
          setCurrentSource(src);
        },
        doSceneQuery,
        doSceneCascadeSearch,
        doSceneFragmentScrape,
        doMultiSceneFragmentScrape,
        stopMultiScrape,
        createNewTag,
        createNewPerformer,
        linkPerformer,
        createNewStudio,
        updateStudio: updateExistingStudio,
        linkStudio,
        updateTag: updateExistingTag,
        resolveScene,
        saveScene,
        submitFingerprints,
        pendingFingerprints: getPendingFingerprints(),
        registerSaveCallback,
        unregisterSaveCallback,
        doBatchSave,
        stopBatchSave,
        loadingBatchSave,
        batchSaveProgress,
        batchSaveCount,
        doBatchSearch,
        stopBatchSearch,
        loadingBatchSearch,
        batchSearchProgress,
      }}
    >
      {children}
    </TaggerStateContext.Provider>
  );
};
