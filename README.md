# build-agent-smith

```mermaid
flowchart TD
    TRIGGER([Scheduler / Webhook\nCron weekly + PR merge + Slack]) --> SUPERVISOR

    subgraph AGENT ["LangGraph Platform Audit Agent"]
        SUPERVISOR[Supervisor Node\nPlan scope · Route workers · Manage state]

        SUPERVISOR --> REPO[Repo Scanner Node\nCI config presence\nDockerfile standards\nBranch protection\nTemplate drift score]

        SUPERVISOR --> CICD[CI/CD Auditor Node\nPipeline YAML parser\n6-dimension maturity rubric\nSecret hygiene scan]

        SUPERVISOR --> COV[Coverage Analyzer Node\nCodecov · JaCoCo fetch\nLine/branch pct\nChange risk correlation]

        SUPERVISOR --> FINOPS[Cloud FinOps Node\nCost Explorer API\nEC2/RDS utilization\nOrphaned resource detection\nUntagged resource flags]

        REPO --> AGG
        CICD --> AGG
        COV --> AGG
        FINOPS --> AGG

        AGG[Findings Aggregator Node\nMerge · Deduplicate\nDORA scoring model\nRank by cost × severity ÷ effort]
    end

    subgraph SOURCES ["External Data Sources"]
        GH[GitHub / GitLab API]
        AWS_CE[AWS Cost Explorer]
        CW[CloudWatch Metrics]
        CC[Codecov API]
    end

    REPO -.->|read-only token| GH
    CICD -.->|YAML fetch| GH
    COV -.->|coverage reports| CC
    FINOPS -.->|GetCostAndUsage| AWS_CE
    FINOPS -.->|CPU/Memory metrics| CW

    subgraph OUTPUT ["Output Layer"]
        PG[(PostgreSQL\nfindings · repo_scores\naudit_runs)]
        MV[Materialized View\nplatform_health_current]
        SLACK[Slack Digest\nWeekly summary\nCritical alerts]
        DASH[Grafana Dashboard\nDORA scores · Cost waste\nCI maturity trends]
    end

    AGG --> OUT[Output Writer Node]
    OUT --> PG
    PG --> MV
    OUT --> SLACK
    MV --> DASH

    classDef node fill:#2d3748,stroke:#4a5568,color:#e2e8f0
    classDef source fill:#1a365d,stroke:#2b6cb0,color:#bee3f8
    classDef output fill:#1c4532,stroke:#276749,color:#c6f6d5
    classDef trigger fill:#44337a,stroke:#6b46c1,color:#e9d8fd

    class SUPERVISOR,REPO,CICD,COV,FINOPS,AGG,OUT node
    class GH,AWS_CE,CW,CC source
    class PG,MV,SLACK,DASH output
    class TRIGGER trigger
```

Built LangGraph-based platform audit agent that automatically measured infrastructure standardization, CI/CD maturity, and test coverage across all 28 repositories — gave engineering leadership and FinOps a continuous view into cloud resource utilization patterns and surfaced $120K+ in underutilized compute that was subsequently right-sized.
