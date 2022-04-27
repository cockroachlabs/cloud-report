# cloud-report-2022

Details about and from the Cloud Report done in Q2 2022

From 2018 to 2021, we produced the series of "Cloud Report", wherein we assessed the performance of AWS vs. GCP along dimensions like TPC-C throughput, I/O, CPU and latency.

This repo will let us aggregate and share data among team members, including processes and results. 

Resources for Cloud Report:
* [Highlights from the 2022 Cloud Report](https://www.cockroachlabs.com/blog/2022-cloud-report/)
* [2022 Cloud Report](https://www.cockroachlabs.com/guides/2022-cloud-report)

## Enclosed Binary

The go program contained in this repo can automatically run tests on cloud providers baked into Roachprod.

For more details, see `reproduction-steps.md` in this repo.

_Note_: It would be possible to extend this binary to run on other platforms relatively easily, but requires some work to handle cloud-specific tasks––namely, getting machine metadata.

## TPC-C Reproduction Steps

* follow the steps in [`reproduction-steps.md`](reproduction-steps.md)
* execute `run_tpcc_experiment.sh`; e.g., `./run_tpcc_experiment.sh -c aws`

## Staff

**Keith McClellan** 
- Vision, structure, messaging, writing

**Charlie Custer** for Marketing
- Vision, structure, messaging, writing

**Jessica Edwards** for Marketing
- Report production and promotion, messaging

**Lidor Carmel** for Engineering
- Automation, data collection and analysis

**Yevgeniy Miretskiy** for Engineering 
- Automation, data collection and analysis

**Stan Rosenberg** for Engineering
- Automation, data collection and analysis

**Jane Xing** for Engineering
- Automation, data collection and analysis
