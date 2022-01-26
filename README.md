# pyshpool
`pyshpool` is for multiprocessing (especially using multiple nodes in parallel) in HPC by shell/python! Welcome aboard!

The current release version for pyshpool is v2.2 and v2.5.1 for pyshNode-static. Arm with pyshpool, feel free to enjoy yourself.

<!-- https://cdn.pixabay.com/photo/2016/03/09/15/28/water-1246669_1280.jpg -->
<p align="center">
  <!-- <img src="https://cdn.pixabay.com/photo/2016/03/09/15/28/water-1246669_1280.jpg" alt="Fish pool" width="400" > -->
    <img src="src/fishpool_free.jpeg"
    title="Fish pool"
    alt="copyright by https://wallwiz.com/share/marine-life/koi-carp-in-the-pond"
    width="400" >
</p>

# Table of Contents

* [Released version](#Released-version)

> * [pypool V2.2](#python-version-V22)
> * [shpool V2.2](#shell-version-V22)
> * [pyshpool-static](#pyshpool-static-version)
> * [pyshNodes-static](#pyshNode-static-version)

* [Quick start](#How-to-run-pyshpool-in-two-steps)
* [Tasklist/joblist](#What-is-input-job-list)
* [*Hand-on example*](#Hand-on-example-on-running-pyshNode)

```
├── LICENSE
├── README.md
├── demoScipt.sh
├── inpTaskList.dat
├── poolRunShell-static
├── poolRunShell-v2.2
├── previousVersion
│   ├── pyshNodesV22-static
│   ├── pyshNodesV23-static
│   ├── pyshNodesV24-static
│   └── pyshNodesV242-static
├── pyshNodeV251-static
├── shellPoolv2.2
└── src
    └── fishpool_free.jpeg
```


# Released version

- python version V2.2

To use the python, you need to make sure that `multiprocessing` is installed in your python environment. To install the `multiprocessing`, please turn to [this site](https://pypi.org/project/multiprocess/).

- shell version V2.2

There is no prerequisite for shell version. U can enjoy pyshpool directly.

- pyshpool static version

You don't have to install any prerequisites for the pyshpool-static version and you just run the codes by one line command to enjoy yourself.

- pyshNode static version (***Updated: VersionV2.5.1 2022-01-24***)

You don't have to install any prerequisites for the pyshNode version and you just run the codes by one line command to enjoy yourself. ***Make sure pyshpool-static is in the same folder with pyshNode-static***. The command is quite similar to slurm script.

```
onelineCommand:
./pyshOnNodes
-p <str:partitionName>[general]
-J <str:>[multiNode]
-N <int:nodeNumber>[2]
-n <int:taskPerNode>[24]
-e <str:email>[NULL]
-w <str:nodelist>[NULL]
-i <inpTaskList>[inpTaskList.dat]

details:

./pyshOnNodes
--partition[-p] <str:partitionName>[general]
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2]
--ntasks[-n] <int:taskPerNode>[24]
--mail-user[-e] <str:email>[NULL]
--nodelist[-w] <str:nodelist>[NULL]
--inputTaskList[-i] <inpTaskList>[inpTaskList.dat]

These varivables are the same with sbatch
--partition[-p] <str:partitionName>[general]
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2]
--ntasks[-n] <int:taskPerNode>[24]
--nodelist[-w] <str:nodelist>[NULL]
--mail-user[-e] <str:email>[NULL]

<inpTaskList> is a list of your tasks and please try to
use the absolute path in the list.
```

Updated in 2022-01-24: Remove the two default settings ACCOUNT, QOS. They are very strong and may lead to errors in other hpc platform.

# How to run pyshpool in two steps?
Without further description, you can enjoy multiprocess freely in HPC ([high performance computing](https://www.netapp.com/data-storage/high-performance-computing/what-is-hpc/)) by shell/python in two steps.
> 1. make sure pyshpool is executive

`chmod 777 pyshpool`

> 2. run the one-line-command to enjoy yourself

`./pyshpool -c CPUNUM -i inputList`

# What is input job list?
The input job list is your job command delimited by '\n'. For detailed information, please use help information or the document. The input job list or the running task list may be like the followings:

```
# command + args (outputs/logs)
bash demoScript.sh 1 > log1.log
bash demoScript.sh 2 > log2.log
bash demoScript.sh 3 > log3.log
bash demoScript.sh 4 > log4.log
bash demoScript.sh 5 > log5.log
bash demoScript.sh 6 > log6.log
bash demoScript.sh 7 > log7.log
bash demoScript.sh 8 > log8.log
bash demoScript.sh 9 > log9.log
bash demoScript.sh 10 > log10.log
```

Notes (Updated in 2022-01-24): If you are using some dependencies to run your programs, the absolute path for the command is recommended. Alternatively, you can use the binary dependncies without absolute path or load your environments in the same line as your programs.

# Hand-on example on running pyshNode

If you are familiar with parallel/xargs/tee command, you will have very little time to command on pyshNode.

1. Clone this github

`git clone https://github.com/jligm-hash/pyshpool.git`

2. Check the executive pyshpool and the demo script list

`chmod 777 poolRunShell-static; chmod 777 pyshNodes-static`
`cat demoScript.sh`
`head inpTaskList.dat`

You will get
```
#!/bin/bash
RANDOM=$1
seudoRunningTime=$((1 + $RANDOM % 10))
echo "Running program $seudoRunningTime s"
sleep $seudoRunningTime
```
and
```
bash demoScript.sh 1 > log1.log
bash demoScript.sh 2 > log2.log
bash demoScript.sh 3 > log3.log
bash demoScript.sh 4 > log4.log
bash demoScript.sh 5 > log5.log
bash demoScript.sh 6 > log6.log
bash demoScript.sh 7 > log7.log
bash demoScript.sh 8 > log8.log
bash demoScript.sh 9 > log9.log
bash demoScript.sh 10 > log10.log
```

3. Run one-command pyshNode and enjoy your self

`./pyshOnNodes -p general -J demoPyshNodes -N 2 -n 24 -i inpTaskList.dat # forHpc2`

`./pyshOnNodes -p cpu-share -J demoPyshNodes -N 2 -n 40 -i inpTaskList.dat # forHpc3`

# How to develop the pyshpool?

Just email in github@jligm-hash to give you access.
