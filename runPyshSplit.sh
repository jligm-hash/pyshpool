#!/bin/bash

# original_file="$1"
# num_files="$2"
# output_format="$1"

# # Get the total number of lines in the original file
# total_lines=$(wc -l < "$original_file")

# # Calculate the number of lines per file
# lines_per_file=$((total_lines / num_files))

# # Split the file into equal-sized chunks
# split -l "$lines_per_file" "$original_file" "tmp_file_"

# # Rename and move the output files
# for file in $(ls tmp_file_*); do
#     file_index=$(echo "$file" | awk -F'_' '{print $NF}')
#     new_file="${output_format}.${file_index}.dat"
#     mv "$file" "$new_file"
# done

# input the file name for the test dat
# input the number of the nodes for the single-node jobs
# or the number of the jobs to be splited

inpDL="$1"
nNode="$2"

inpDlHeader="$inpDL"

partition='x-gpu-share'
jobName='pyshBatch'

totalLineNumber=$(cat $inpDL | wc -l)
totalFileNumber=$nNode

linesPerFile=$((($totalLineNumber + $totalFileNumber - 1)/$totalFileNumber))



for i in $(seq 1 $nNode)
do
    startNum=$((($i-1)*$linesPerFile))
    endNum=$(($i*$linesPerFile))
    if  [ $endNum -le $totalLineNumber ]
    then
        head -n $endNum $inpDL | tail -n $linesPerFile > ${inpDlHeader}.sub.${i}.out
    else
        lessLine=$(($linesPerFile-$endNum+$totalLineNumber))
        echo "Note: $lessLine tasks on the last node." >> ${logOut}
        head -n $totalLineNumber $inpDL | tail -n $lessLine > ${inpDlHeader}.sub.${i}.out
    fi
    
    echo "#!/bin/bash
#SBATCH -p $partition
#SBATCH -J ${jobName}${i}
#SBATCH --nodes=1

#SBATCH --exclusive
#SBATCH --mem=60G
#SBATCH --gres=gpu:1
#SBATCH --gres-flags=enforce-binding
#SBATCH --mail-type=all
##SBATCH --mail-user=
 

##SBATCH --gres-flags=rtx6000:1
##SBATCH --time=1:00:00
##SBATCH --cpus-per-task=$nCpu
##SBATCH -n $nNode

./poolRunShell-static -i ${inpDlHeader}.sub.${i}.out
" > ${inpDlHeader}.sub.${i}.slurmDat

sbatch ${inpDlHeader}.sub.${i}.slurmDat

done


# use the for-loop to generate the slurm script and submit by sbatch



