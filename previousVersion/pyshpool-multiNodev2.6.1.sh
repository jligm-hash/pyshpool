#!/bin/bash
partition='general'
jobName='multiNode'
nNode=1
nCpu=128
email="NULL"
nodelist="NULL"
inpDL="inpTaskList.dat"

logOut=$(echo pyshNode $(date) out | tr ' ' '.' | tr ':' '-')


# --------------------------------------------------------------------------------
# STEP 1: set the input parameter
echo "
-----------------------------------------------------------------
Easy Pyshpool Logo for multi-node V2.6.1
-----------------------------------------------------------------
***THIS IS WELCOME CENTER***
Welcome to the world of PYSHPOOL. To look for more information, please
turn to github::pyshpool for more information.

Basic usage (version v2.6.1): 

./pyshOnNodes 
-p <str:partitionName>[general] 
-J <str:>[multiNode]
-N <int:nodeNumber>[2] 
-n <int:taskPerNode>[24]
-e <str:email>[NULL]
-w <str:nodelist>[NULL]
-i <inpTaskList>[inpTaskList.dat]
-A <str:>[accountName]


To get the help message: ./pyshOnNodes --help[-h]
-----------------------------------------------------------------
" > ${logOut}

helpmessage=$( cat <<EOF
***THIS IS HELPING CENTER***

Make sure you have poolRunShell-static already. To get it individually, try
wget https://github.com/jligm-hash/pyshpool/raw/main/poolRunShell-static

./pyshOnNodes 
--partition[-p] <str:partitionName>[general] 
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2] 
--ntasks[-n] <int:taskPerNode>[24]
--mail-user[-e] <str:email>[NULL]
--nodelist[-w] <str:nodelist>[NULL]
--inputTaskList[-i] <inpTaskList>[inpTaskList.dat]
--account[-A] <str:>[accountName]

These varivables are the same with sbatch
--partition[-p] <str:partitionName>[general] 
--job-name[-J] <str:>[multiNode]
--nodes[-N] <int:nodeNumber>[2] 
--ntasks[-n] <int:taskPerNode>[24]
--mail-user[-e] <str:email>[NULL]
--nodelist[-w] <str:nodelist>[NULL]

<inpTaskList> is a list of your tasks and please try to
use the absolute path in the list.
Example:
bash job1.sh > job1.log
bash job2.sh > job2.log
python job3.py > job3.log

EOF
)

### getopts
while [ $# -gt 0 ]; do
	if [  "$1" == "-h" -o "$1" == "-help" -o "$1" == "--help" ]; then
		shift;
		echo '---------------------------------------------------------------------' >> ${logOut}
		echo "$helpmessage" >> ${logOut}
        cat ${logOut}
        rm ${logOut}
		exit;
	elif [  "$1" == "-p" -o "$1" == "--partition" ]; then
		shift;
		partition=$1;
		shift
    elif [  "$1" == "-J" -o "$1" == "--job-name" ]; then
		shift;
		jobName=$1;
		shift
	elif [  "$1" == "-N" -o "$1" == "--nodes" ]; then
		shift;
		nNode=$1;
		shift
    elif [  "$1" == "-n" -o "$1" == "--ntasks" ]; then
		shift;
		nCpu=$1;
		shift
	elif [  "$1" == "-e" -o "$1" == "--mail-user" ]; then
		shift;
		email=$1;
		shift
    elif [  "$1" == "-w" -o "$1" == "--nodelist" ]; then
		shift;
		nodelist=$1;
		shift
	elif [  "$1" == "-i" -o "$1" == "--inputTaskList" ]; then
		shift;
		inpDL=$1;
		shift
	elif [  "$1" == "-A" -o "$1" == "--account" ]; then
		shift;
		accountName=$1;
		shift
	else	# if unknown argument, just shift
		shift
	fi
done

echo '---------------------------------------------------------------------
***THIS IS INFORMATION CENTER***' >> ${logOut}
echo "Running pyshool-multinode in $PWD" >> ${logOut}
echo "Current partition is [$partition], 
jobName is [$jobName],
number of nodes is [$nNode], 
number of tasks on one node is [$nCpu]." >> ${logOut}


# --------------------------------------------------------------------------------
# STEP 2: split the inputTaskList inpDL into inpDL${num}.dat
totalLineNumber=$(cat $inpDL | wc -l)
totalFileNumber=$nNode

linesPerFile=$((($totalLineNumber + $totalFileNumber - 1)/$totalFileNumber))

echo "The total task number is [$totalLineNumber]," >> ${logOut}
echo "total node number is [$totalFileNumber]," >> ${logOut}
echo "[$linesPerFile] tasks per node." >> ${logOut}

for i in $(seq 1 $nNode)
do
    startNum=$((($i-1)*$linesPerFile))
    endNum=$(($i*$linesPerFile))
    if  [ $endNum -le $totalLineNumber ]
    then
        head -n $endNum $inpDL | tail -n $linesPerFile > inpDL${i}.out
    else
        lessLine=$(($linesPerFile-$endNum+$totalLineNumber))
        echo "Note: $lessLine tasks on the last node." >> ${logOut}
        head -n $totalLineNumber $inpDL | tail -n $lessLine > inpDL${i}.out
    fi
done

# if the task number is zero, then exit
if [ $totalLineNumber -eq 0 ] 
then
  echo "The total task number is ZERO!" >> ${logOut}
  echo '---------------------------------------------------------------------' >> ${logOut}
  echo "NOTES: please check your task list!" >> ${logOut}
  cat ${logOut}
  rm ${logOut}
  exit
fi

# --------------------------------------------------------------------------------
# STEP 3: create the sbatch script

echo "#!/bin/bash
#SBATCH -p $partition
#SBATCH -J $jobName
#SBATCH --nodes=$nNode
#SBATCH -n $nNode
#SBATCH --cpus-per-task=$nCpu
#SBATCH -A $accountName
#SBATCH --exclusive
" > tmp.git

# remove the following lines ^
#SBATCH --account=hpcusers
#SBATCH --qos=hpcusers


if [ "$email" = "NULL" ]
then
  echo "The email is NULL!"
else
  echo "#SBATCH --mail-user=$email
#SBATCH --mail-type=all" >> tmp.git
fi

if [ "$nodelist" = "NULL" ]
then
  echo "The nodelist is NULL!"
else
  echo "#SBATCH --nodelist=$nodelist" >> tmp.git
fi

echo '---------------------------------------------------------------------'  >> ${logOut}



echo "cd $PWD" >> tmp.git

for i in $(seq 1 $nNode)
do
    echo "srun --nodes=1 --ntasks=1 --cpus-per-task=$nCpu ./poolRunShell-static -c $nCpu -i inpDL${i}.out > logNode${i}.out &" >> tmp.git
done

echo "
wait" >> tmp.git


# --------------------------------------------------------------------------------
# STEP 4: run the sbatch

sbatch tmp.git >> ${logOut}
rm tmp.git
# rm inpDL*out

cat ${logOut}