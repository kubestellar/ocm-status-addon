#!/bin/bash
# add helm headers to cluster scoped files such as ClusterRole and CLusterRoleBinding
# to avoid helm trying to create a copy of the each clusterrClusterRoleole or CLusterRoleBinding
# for each instance of status-addon in a different namespace.
HOME_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

if [ "$#" -lt 1 ]; then
    echo "adds/remove helm tags to cluster scoped files such as ClusterRole and CLusterRoleBinding"
    echo "Usage: $(basename $0) add | remove"
    exit
fi
CMD=$1

cleanup_helm_tags() {
  file=$1
  sed -i.bak '/^#{{-/d' $file
  rm ${file}.bak
}


# Directory containing the yaml file
dir=${HOME_DIR}/chart/templates

# file to process
op_file=${dir}/operator.yaml

cleanup_helm_tags $op_file

if [[ $CMD == "add" ]]; then
  # Split the file into separate YAML files based on the separator
  TMP_DIR=$(mktemp -d -t status-addon)
  c=1
  IFS=''
  while read line; do
    #echo $line
    if [[ $line == "---" ]]; then
      c=$((c+1))
    else
      echo "$line" >> "${TMP_DIR}/${c}.yaml"
    fi
  done < "$op_file"

  # Loop over all yaml files in the directory
  for file in $TMP_DIR/*.yaml; do
    # Extract the kind and name from the yaml file
    kind=$(yq e '.kind' $file)
    name=$(yq e '.metadata.name' $file)

    # Check if the kind is ClusterRole or ClusterRoleBinding
    if [[ $kind == "ClusterRole" ]] || [[ $kind == "ClusterRoleBinding" ]]; then
      #echo processing $name
      # need unique ClusterRole or ClusterRoleBinding for each instance
      sed -i.bak "s/${name}/'{{.Values.ControlPlaneName}}-${name}'/g" $file
      rm ${file}.bak
      if [[ $kind == "ClusterRoleBinding" ]]; then
        #echo processing $name
        # adjust reference
        ref=$(yq e '.roleRef.name' $file)
        echo $ref
        yq eval '.roleRef.name |= "{{.Values.ControlPlaneName}}-'${ref}'"' $file -i
      fi
    fi  
  done

  # Loop over all yaml files in the directory and append back to op_file
  rm $op_file
  for file in $TMP_DIR/*.yaml; do
    echo "---" >> $op_file
    cat $file >> $op_file
  done
fi  

rm -rf $TEMP_DIR




