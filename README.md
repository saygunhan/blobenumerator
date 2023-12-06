# blobenumerator
Azure Blob Enumerator written in go. Based on the file "perm.txt" this tool brute forces azure account names to find if they exist. After it is completed the tool proceeds with enumerating public containers with the same technique. After the completion of container enumeration the tool extracts the publicly shared data
# Usage
go run v2.go -b <yourkeywordhere> -o your_output_file.txt
