import argparse
import socket
import requests as req
import xml.etree.ElementTree as et
import time  # Import the time module
import threading

lock = threading.Lock()
# Create a parser object
parser = argparse.ArgumentParser(description='Check if a URL is accessible.')
# Add a command-line argument for the URL using a flag
parser.add_argument('-b', '--base', type=str, required=True, help='The URL to check.')
parser.add_argument('-w', '--wordlist', type=str, required=False, help='The wordlist for petmutation. By default it will use perm.txt Eg: /usr/share/wordlists/myperm.txt')
parser.add_argument('-v', '--verbose', action='store_true', required=False, help='Verbosity.')
parser.add_argument('-o', '--output', type=str, required=False, help='Output to a file.')
# Parse the command-line arguments
args = parser.parse_args()


blobUrls = {
    'onmicrosoft.com':'Microsoft Hosted Domain',
    'scm.azurewebsites.net': 'App Services - Management',
    'azurewebsites.net': 'App Services',
    'p.azurewebsites.net': 'App Services',
    'cloudapp.net': 'App Services',
    'file.core.windows.net': 'Storage Accounts - Files',
    'blob.core.windows.net': 'Storage Accounts - Blobs',
    'queue.core.windows.net': 'Storage Accounts - Queues',
    'table.core.windows.net': 'Storage Accounts - Tables',
    'mail.protection.outlook.com': 'Email',
    'sharepoint.com': 'SharePoint',
    'redis.cache.windows.net': 'Databases-Redis',
    'documents.azure.com': 'Databases-Cosmos DB',
    'database.windows.net': 'Databases-MSSQL',
    'vault.azure.net': 'Key Vaults',
    'azureedge.net': 'CDN',
    'search.windows.net': 'Search Appliance',
    'azure-api.net': 'API Services',
    'azurecr.io': 'Azure Container Registry',
    'servicebus.windows.net': 'Service Bus',
}
keys_list = list(blobUrls.keys())

permutations = []
activeAccounts = []
activeContainers = []
foundFiles = []

if args.wordlist is  None:
    wordlist = "perm.txt"
    print("Using default wordlist")
    contents = open(wordlist, 'r')
    permutations = contents.readlines()
    for i in range(len(permutations)):
        permutations[i] = permutations[i].strip()
else:
    wordlist = args.wordlist
    contents = open(wordlist, 'r')
    permutations = contents.readlines()
    for i in range(len(permutations)):
        permutations[i] = permutations[i].strip()



def is_domain_active(domain):
    try:
        # Resolve the domain's IP address
        ip_address = socket.gethostbyname(domain)
        return ip_address
    except socket.gaierror:
        return None

# Check if the sotorage account is active
def accountCheck():
    #Check if the domain is active by using only base
    for bloburl in range (len(keys_list)):
        naked = args.base + "." + keys_list[bloburl]
        ip_address = is_domain_active(naked)
        if ip_address is not None:
            print(f"The account {naked} is active with IP address {ip_address}.")
            activeAccounts.append(naked)
        else:
            if args.verbose is None:
                print(f"The account {naked} is not active.")
            pass
    # Check if the domain is active by using wordlist
    for bloburl in range (len(keys_list)):
        for perm in range(len(permutations)):
            full = permutations[perm] + args.base + "." + keys_list[bloburl]
            ip_address = is_domain_active(full)
            if ip_address is not None:
                print(f"The account {full} is active with IP address {ip_address}.")
                activeAccounts.append(full)
            else:
                if args.verbose is None:
                    print(f"The account {full} is not active.")
                continue

def containerFiles(uri):
    response = req.get(uri + "?restype=container&comp=list")
    root = et.fromstring(response.content)
    for name in root.findall(".//Blob/Name"):
        foundFiles.append(uri+name.text)


def containerCheck(account):
    for perm in range(len(permutations)):
        try:
            dir = account + "/" + permutations[perm]
            uri = "https://" + dir + "?restype=container&comp=list"
            response = req.get(uri)
            if response.status_code == 200:
                with lock:
                    activeContainers.append("https://" + dir + '/')
            else:
                if args.verbose is None:
                    with lock:
                        print(f"{uri} is not active")
                continue
        except Exception as e:
            print(e)
            pass

def start():
    print("Starting")
    accountCheck()
    print("########## Active Accounts ##########")
    print(activeAccounts)

    # Use threading for concurrent container checking
    container_threads = []
    for account in activeAccounts:
        t = threading.Thread(target=containerCheck, args=(account,))
        container_threads.append(t)
        t.start()

    for t in container_threads:
        t.join()  # Wait for all container threads to finish

    print("########## Active Containers ##########")
    print(activeContainers)

    print("########## Files Found ##########")
    for uri in activeContainers:
        containerFiles(uri)
    print(foundFiles)

    if args.output is not None:
        with open(args.output, "w") as f:
            for i in range(len(foundFiles)):
                f.write(foundFiles[i] + "\n")

    return foundFiles, activeAccounts, activeContainers

if __name__ == "__main__":
    start_time = time.time()
    start()
    end_time = time.time()  # Record the end time
    elapsed_time = end_time - start_time
    print(f"Script execution time: {elapsed_time:.2f} seconds")