## Microbenchmarks

### Run on GCP & AWS

To test all machines, run:
~~~
./cloud-report-2019
~~~

Meaningful flags inlcude:
** Flag ** | ** Operation **
-----------|----------------
`-cloudDetails` | Specify a JSON file to detail the machine types you want to test. Use `cloudDetails/default.json` as a template. <br/><br/>For any machines you want to test with EBS on AWS, make sure they're listed as `ebsMachineTypes`.
`-skipio` | Skip the IO tests, which take a long time to complete
`-iteration` | Run the benchmark tests _x_ times against the same machines. To run the tests against a separate set of machines, you must manually destroy the roachprod cluster that gets created.


As noted above, you can choose some other set of machines to test by specifying another file with `-cloudDetails`.

### Run on Azure

1. Manually provision the machines you want to test on Azure, with two crucial considerations:

    - The user name must match `whoami`
    - The SSH key must be `~/.ssh/id_rsa.pub`
    
2. Run

    ~~~
    ./cloud-report-2019 -azure -node1 <public IP of node 1> -node2 <public IP of node 2>
    ~~~

### Results

Results for each benchmark are automatically saved and parsed into CSVs in the `results` folder. How we'll ingest these into Google Sheets is TBD.
