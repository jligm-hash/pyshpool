#!/bin/bash

# --------------------------------------------------------------------------------
# STEP 1: set the input parameter
# define the cpu number
cpuNum=3
input="inputShellScript.dat"

start_time=`date +%s`              # Define the start time of the script run

echo "
-----------------------------------------------------------------
██████╗░██╗░░░██╗░██████╗██╗░░██╗██████╗░░█████╗░░█████╗░██╗░░░░░
██╔══██╗╚██╗░██╔╝██╔════╝██║░░██║██╔══██╗██╔══██╗██╔══██╗██║░░░░░
██████╔╝░╚████╔╝░╚█████╗░███████║██████╔╝██║░░██║██║░░██║██║░░░░░
██╔═══╝░░░╚██╔╝░░░╚═══██╗██╔══██║██╔═══╝░██║░░██║██║░░██║██║░░░░░
██║░░░░░░░░██║░░░██████╔╝██║░░██║██║░░░░░╚█████╔╝╚█████╔╝███████╗
╚═╝░░░░░░░░╚═╝░░░╚═════╝░╚═╝░░╚═╝╚═╝░░░░░░╚════╝░░╚════╝░╚══════╝
-----------------------------------------------------------------

Welcome to the world of PYSHPOOL. To look for more information, please
turn to github::pyshpool for more information.

Basic usage: ./shellPool -c <int:cpuNumber>[3] -i <inputShellScriptList>
[inputShellScript.dat]

To get the help message: ./shellPool --help[-h]
-----------------------------------------------------------------
"


# echo '---------------------------------------------------------------------'

helpmessage=$( cat <<EOF

./shellPool -c <int:cpuNumber>[3] -i <inputShellScriptList>[inputShellScript.dat]

<int:cpuNumber> to use the max cpu: use '-c max' or '-c 0'
[this is not available in shell version]
<inputShellScriptList> Example:
bash job1.sh > job1.log
bash job2.sh > job2.log
python job3.py > job3.log
EOF
)

### getopts
while [ $# -gt 0 ]; do
	if [  "$1" == "-h" -o "$1" == "-help" -o "$1" == "--help" ]; then
		shift;
		echo '---------------------------------------------------------------------'
		echo "$helpmessage"
		exit;
	elif [  "$1" == "-c" -o "$1" == "--cpuNumber" ]; then
		shift;
		cpuNum=$1;
		shift
	elif [  "$1" == "-i" -o "$1" == "--inputShellScriptList" ]; then
		shift;
		input=$1;
		shift
	else	# if unknown argument, just shift
		shift
	fi
done

echo "Current cpu num is ${cpuNum}, current input file is ${input}"

# --------------------------------------------------------------------------------
# STEP 2: build up the pool

namedPipe=./fd1 # define the local named pipe
[ -e ${namedPipe} ] || mkfifo ${namedPipe} # Create a well-known pipe
exec 3<>${namedPipe}                   # Create a file descriptor and associate the pipe file in a readable (<) writable (>) way. At this time, file descriptor 3 has all the characteristics of a well-known pipe file
rm -rf ${namedPipe}                    # The associated file descriptor has all the characteristics of the pipeline file, so the pipeline file can be deleted at this time, and we can leave the file descriptor for use.

# define the pool size by the cpu number
for ((i=1;i<=$cpuNum;i++))
do
        echo >&3                   # &3 stands for referencing file descriptor 3. This command stands for putting a "token" into the pipeline
done


# --------------------------------------------------------------------------------
# STEP 3: run the shell list in the pool
# to create the process
for ((i=1;i<=$cpuNum;i++))
do
read -u3                           # Represents reading a token from the pipeline
{
        echo 'Initial success #'$i
        echo >&3                   # On behalf of me this time the command is executed to the end, put the token back into the pipeline
}&
done
wait

# input and run the input shell list
count=0
while IFS= read -r line
do
# echo "$line"
#   $line
        let count++
        read -u3                           # Represents reading a token from the pipeline
        {
                echo "$line"  # sleep 1 is used to imitate the time it takes to execute a command (can be replaced by a real command)
                eval $line

                echo 'success' $count
                echo >&3                   # On behalf of me this time the command is executed to the end, put the token back into the pipeline
        }&

        # if [[ `expr $count % 10` -eq 0 ]]; then
        #     break
        # fi
done < "$input"
wait


# --------------------------------------------------------------------------------
# STEP 4: end the runs, print the logs, close the files
stop_time=`date +%s`  # define the stop time of the script run

#
ELAPSED="Time used: $(($SECONDS / 3600)) hrs $((($SECONDS / 60) % 60)) min $(($SECONDS % 60)) sec"
echo '---------------------------------------------------------------------'
echo $ELAPSED
echo "Successfully run ${count} jobs"
echo "Current cpu num is ${cpuNum}, current input file is ${input}"
echo '---------------------------------------------------------------------'
# turn off the reading of file
exec 3<&-                       # Turn off reading of file descriptors
exec 3>&-                       # Turn off writing of file descriptors
