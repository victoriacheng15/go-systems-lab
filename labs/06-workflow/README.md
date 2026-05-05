# 06 workflow

`workflow` is the systems capstone of this lab. It synthesizes the primitives from Labs 01-05 into a functional **Event-Driven Orchestrator**—a "Mini-GitHub Actions" that runs locally in your terminal.

## The Integrated Lifecycle

This lab demonstrates how independent kernel features work together to create a platform:

1.  **Detection (`inotify`)**: The runner watches the `labs/06-workflow/jobs/` directory. Dropping a script file there triggers a new job.
2.  **Governance (`cgroups`)**: Every job is automatically placed into a Cgroup v2 with a strict memory budget (e.g., 50MB) to protect the host.
3.  **Observation (`procfs`)**: While the job runs, the runner polls `/proc/[PID]/stat` to calculate real-time CPU and Memory consumption.
4.  **Lifecycle (`signals`)**: The runner manages job timeouts. If a job hangs, it is terminated via `SIGTERM`. If the runner is stopped, it gracefully cleans up all active jobs.

## Manual Usage

Run from the repository root:

1.  **Start the Workflow Runner:**
    ```bash
    sudo go run labs/06-workflow/main.go
    ```

2.  **Submit a Job (In another terminal):**
    ```bash
    # Create a simple job that uses some resources
    echo "sleep 5 && echo 'Job Finished'" > labs/06-workflow/jobs/test.sh
    chmod +x labs/06-workflow/jobs/test.sh
    ```

3.  **Watch the Orchestration:**
    The runner terminal will show the detection, the PID assignment, live telemetry, and the final cleanup.

## 📖 Reference: The Orchestrator "Control Loop"

In systems engineering, an orchestrator is essentially an infinite "Control Loop." It constantly compares the **Desired State** (a job file exists) with the **Current State** (the job is not running) and takes action to reconcile them. This lab uses `inotify` as the "Edge Trigger" for that loop, ensuring the runner only consumes CPU when there is actual work to be done.

[Back to main README](../../README.md)
