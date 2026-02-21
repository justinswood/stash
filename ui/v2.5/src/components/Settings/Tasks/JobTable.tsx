import {
  faBan,
  faCheck,
  faCircle,
  faCircleExclamation,
  faCog,
  faHourglassStart,
  faTimes,
} from "@fortawesome/free-solid-svg-icons";
import React, { useEffect, useRef, useState } from "react";
import { Button, Card, ProgressBar } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Icon } from "src/components/Shared/Icon";
import {
  mutateStopJob,
  useJobQueue,
  useJobsSubscribe,
} from "src/core/StashService";
import * as GQL from "src/core/generated-graphql";

type JobFragment = Pick<
  GQL.Job,
  | "id"
  | "status"
  | "subTasks"
  | "description"
  | "progress"
  | "error"
  | "startTime"
>;

function formatDuration(ms: number): string {
  const totalSeconds = Math.round(ms / 1000);
  if (totalSeconds < 60) return `${totalSeconds}s`;
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
}

interface IJob {
  job: JobFragment;
}

const WINDOW_MS = 60000; // Use last 60 seconds of progress for rate calculation
const MIN_PROGRESS_FOR_ETA = 0.02; // Don't show ETA below 2%
const MIN_SNAPSHOTS = 2; // Need at least 2 data points

const Task: React.FC<IJob> = ({ job }) => {
  const [stopping, setStopping] = useState(false);
  const [className, setClassName] = useState("");
  const progressHistory = useRef<Array<{ time: number; progress: number }>>([]);
  const lastJobId = useRef(job.id);

  // Reset history when job changes
  if (job.id !== lastJobId.current) {
    progressHistory.current = [];
    lastJobId.current = job.id;
  }

  // Record progress snapshots
  useEffect(() => {
    if (
      job.status === GQL.JobStatus.Running &&
      job.progress !== null &&
      job.progress !== undefined &&
      job.progress > 0
    ) {
      const now = Date.now();
      const history = progressHistory.current;

      // Only add if progress actually changed
      if (
        history.length === 0 ||
        history[history.length - 1].progress !== job.progress
      ) {
        history.push({ time: now, progress: job.progress });
      }

      // Trim to sliding window
      const cutoff = now - WINDOW_MS;
      while (history.length > MIN_SNAPSHOTS && history[0].time < cutoff) {
        history.shift();
      }
    }
  }, [job.progress, job.status]);

  useEffect(() => {
    setTimeout(() => setClassName("fade-in"));
  }, []);

  useEffect(() => {
    if (
      job.status === GQL.JobStatus.Cancelled ||
      job.status === GQL.JobStatus.Failed ||
      job.status === GQL.JobStatus.Finished
    ) {
      // fade out around 10 seconds
      setTimeout(() => {
        setClassName("fade-out");
      }, 9800);
    }
  }, [job]);

  async function stopJob() {
    setStopping(true);
    await mutateStopJob(job.id);
  }

  function canStop() {
    return (
      !stopping &&
      (job.status === GQL.JobStatus.Ready ||
        job.status === GQL.JobStatus.Running)
    );
  }

  function getStatusClass() {
    switch (job.status) {
      case GQL.JobStatus.Ready:
        return "ready";
      case GQL.JobStatus.Running:
        return "running";
      case GQL.JobStatus.Stopping:
        return "stopping";
      case GQL.JobStatus.Finished:
        return "finished";
      case GQL.JobStatus.Cancelled:
        return "cancelled";
      case GQL.JobStatus.Failed:
        return "failed";
    }
  }

  function getStatusIcon() {
    let icon = faCircle;
    let iconClass = "";
    switch (job.status) {
      case GQL.JobStatus.Ready:
        icon = faHourglassStart;
        break;
      case GQL.JobStatus.Running:
        icon = faCog;
        iconClass = "fa-spin";
        break;
      case GQL.JobStatus.Stopping:
        icon = faCog;
        iconClass = "fa-spin";
        break;
      case GQL.JobStatus.Finished:
        icon = faCheck;
        break;
      case GQL.JobStatus.Cancelled:
        icon = faBan;
        break;
      case GQL.JobStatus.Failed:
        icon = faCircleExclamation;
        break;
    }

    return <Icon icon={icon} className={`fa-fw ${iconClass}`} />;
  }

  function maybeRenderProgress() {
    if (
      job.status === GQL.JobStatus.Running &&
      job.progress !== undefined &&
      job.progress !== null
    ) {
      const progress = job.progress * 100;
      return (
        <ProgressBar
          animated
          now={progress}
          label={`${progress.toFixed(0)}%`}
        />
      );
    }
  }

  function maybeRenderETA() {
    if (
      job.status !== GQL.JobStatus.Running ||
      job.progress === null ||
      job.progress === undefined ||
      job.progress < MIN_PROGRESS_FOR_ETA
    ) {
      return;
    }

    const history = progressHistory.current;

    let remainingMs: number | undefined;

    if (history.length >= MIN_SNAPSHOTS) {
      // Use sliding window rate for accurate estimate
      const oldest = history[0];
      const newest = history[history.length - 1];
      const timeDelta = newest.time - oldest.time;
      const progressDelta = newest.progress - oldest.progress;

      if (timeDelta > 0 && progressDelta > 0) {
        const rate = progressDelta / timeDelta; // progress per ms
        remainingMs = (1 - job.progress) / rate;
      }
    }

    // Fallback to linear extrapolation from start
    if (remainingMs === undefined && job.startTime) {
      const elapsed = Date.now() - new Date(job.startTime).valueOf();
      remainingMs = (elapsed * (1 - job.progress)) / job.progress;
    }

    if (remainingMs === undefined || remainingMs < 0) return;

    return (
      <span className="job-eta">
        <FormattedMessage id="eta" />: {formatDuration(remainingMs)}
      </span>
    );
  }

  function maybeRenderSubTasks() {
    if (
      job.status === GQL.JobStatus.Running ||
      job.status === GQL.JobStatus.Stopping
    ) {
      return (
        <div>
          {/* eslint-disable react/no-array-index-key */}
          {(job.subTasks ?? []).map((t, i) => (
            <div className="job-subtask" key={i}>
              {t}
            </div>
          ))}
          {/* eslint-enable react/no-array-index-key */}
        </div>
      );
    }

    if (job.status === GQL.JobStatus.Failed && job.error) {
      return <div className="job-error">{job.error}</div>;
    }
  }

  return (
    <li className={`job ${className}`}>
      <div>
        <Button
          className="minimal stop"
          size="sm"
          onClick={() => stopJob()}
          disabled={!canStop()}
        >
          <Icon icon={faTimes} />
        </Button>
        <div className={`job-status ${getStatusClass()}`}>
          <div className="job-description">
            <div>
              {getStatusIcon()}
              <span>{job.description}</span>
            </div>
            {maybeRenderETA()}
          </div>
          <div>{maybeRenderProgress()}</div>
          {maybeRenderSubTasks()}
        </div>
      </div>
    </li>
  );
};

export const JobTable: React.FC = () => {
  const intl = useIntl();
  const jobStatus = useJobQueue();
  const jobsSubscribe = useJobsSubscribe();

  const [queue, setQueue] = useState<JobFragment[]>([]);

  useEffect(() => {
    setQueue(jobStatus.data?.jobQueue ?? []);
  }, [jobStatus]);

  useEffect(() => {
    if (!jobsSubscribe.data) {
      return;
    }

    const event = jobsSubscribe.data.jobsSubscribe;

    function updateJob() {
      setQueue((q) =>
        q.map((j) => {
          if (j.id === event.job.id) {
            return event.job;
          }

          return j;
        })
      );
    }

    switch (event.type) {
      case GQL.JobStatusUpdateType.Add:
        // add to the end of the queue
        setQueue((q) => q.concat([event.job]));
        break;
      case GQL.JobStatusUpdateType.Remove:
        // update the job then remove after a timeout
        updateJob();
        setTimeout(() => {
          setQueue((q) => q.filter((j) => j.id !== event.job.id));
        }, 10000);
        break;
      case GQL.JobStatusUpdateType.Update:
        updateJob();
        break;
    }
  }, [jobsSubscribe.data]);

  return (
    <Card className="job-table">
      <ul>
        {!queue?.length ? (
          <span className="empty-queue-message">
            {intl.formatMessage({ id: "config.tasks.empty_queue" })}
          </span>
        ) : undefined}
        {(queue ?? []).map((j) => (
          <Task job={j} key={j.id} />
        ))}
      </ul>
    </Card>
  );
};
