# cloud-report-2021

Details about and from the Cloud Report done in Q4 2020

In 2018, we produced a "Cloud Report", wherein we assessed the performance of AWS vs. GCP along dimensions like TPC-C throughput, I/O, CPU and latency. This year, we aim to produce a similar report but including Azure.

This repo will let us aggregate and share data among team members, including processes and results. 

* [Highlights from the 2021 Cloud Report](https://www.cockroachlabs.com/blog/2021-cloud-report/)
* [2021 Cloud Report](https://www.cockroachlabs.com/guides/2021-cloud-report)



## Enclosed Binary

The go program contained in this repo can automatically run tests on cloud providers baked into Roachprod.

For more details, see `reproduction-steps.md` in this repo.

_Note_: It would be possible to extend this binary to run on other platforms relatively easily, but requires some work to handle cloud-specific tasks––namely, getting machine metadata.

## TPC-C Reproduction Steps

[This guide](https://www.cockroachlabs.com/docs/stable/performance-benchmarking-with-tpc-c-1k-warehouses.html) from the CockroachDB docs shows you how to run the TPC-C benchmark on CockroachDB to reproduce the TPC-C results in the report.

## Staff

**John Kendall** for Product
- Vision, structure, messaging, writing

**Jessica Edwards** for Marketing
- Report production and promotion, messaging

**Yevgeniy Miretsky** for Engineering
- Data collection and aggregation

**Arul Ajmani** for Engineering
- Data collection and aggregation
