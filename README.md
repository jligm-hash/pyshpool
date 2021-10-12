# pyshpool
pyshpool is for multiprocessing in HPC by shell/python! Welcome aboard!
The current release version is v2.2. Feel free to enjoy yourself.

<!-- https://cdn.pixabay.com/photo/2016/03/09/15/28/water-1246669_1280.jpg -->
<p align="center">
  <img src="https://cdn.pixabay.com/photo/2016/03/09/15/28/water-1246669_1280.jpg" alt="Fish pool" width="400" >
</p>

# Table of Contents
* [pypool](#python-version-V2-'.'-2)
* [shpool](#shell-version-V2-'.'-2)
* [pyshpool-static](#pyshpool-static-version)
* [pyshNodes-static](#pyshNode-static-version)
* [Quick start](#How-to-run-pyshpool-in-2-step-'?')
* [Tasklist/joblist](#What-is-input-job-list-'?')
* [Hand-on example](#Hand-on-example-on-running-pyshNode)


## python version V2.2

To use the python, you need to make sure that `multiprocessing` is installed in your python environment. To install the `multiprocessing`, please turn to [this site](https://pypi.org/project/multiprocess/).

## shell version V2.2

There is no prerequisite for shell version. U can enjoy pyshpool directly.

## pyshpool static version

You don't have to install any prerequisites for the pyshpool-static version and you just run the codes by one line command to enjoy yourself.

## pyshNode static version

You don't have to install any prerequisites for the pyshNode version and you just run the codes by one line command to enjoy yourself. Make sure pyshpool-static is in the same folder with pyshNode-static. The command is quite similar to slurm script.

```
onelineCommand:
./pyshOnNodes -p <str:partitionName>[general]
-J <str:>[multiNode]
-N <int:nodeNumber>[2]
-n <int:taskPerNode>[24]
-e <str:email>[NULL]
-i <inpTaskList>[inpTaskList.dat]

details:

./pyshOnNodes --partition[-p] <str:partitionName>[general]
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2]
--ntasks[-n] <int:taskPerNode>[24]
--mail-user[-e] <str:email>[NULL]
--inputTaskList[-i] <inpTaskList>[inpTaskList.dat]

These varivables are the same with sbatch
--partition[-p] <str:partitionName>[general]
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2]
--ntasks[-n] <int:taskPerNode>[24]
--mail-user[-e] <str:email>[NULL]

<inpTaskList> is a list of your tasks and please try to
use the absolute path in the list.
```

# How to run pyshpool in 2 step?
Without further description, you can enjoy multiprocess freely in HPC ([high performance computing](https://www.netapp.com/data-storage/high-performance-computing/what-is-hpc/)) by shell/python in two steps.
- 1. make sure pyshpool is executive

`chmod 777 pyshpool`

- 2. run the one-line-command to enjoy yourself

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

# Hand-on example on running pyshNode

1. Clone this github

`git clone https://github.com/jligm-hash/pyshpool.git`

2. Check the executive pyshpool and the demo script list

`chmod 777 poolRunShell-static; chmod 777 pyshNodesV22-static`
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
