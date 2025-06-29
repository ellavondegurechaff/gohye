import os
import json
import boto3
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
from multiprocessing import Pool
import re
from collections import Counter

DO_SPACES_KEY = 'DO00942YPCJNT62UAZG6'
DO_SPACES_SECRET = 'kk83FHgUwR+MgtKJsOBYL1Zihjj7d9SEaaTqay8lKY8'
DO_SPACES_REGION = 'sfo2'
DO_SPACES_BUCKET = 'hyejoo'

ACCEPTED_EXTENSIONS = {".jpg", ".png", ".jpeg", ".gif"}  # Add .gif to the set of accepted extensions

session = boto3.session.Session()
s3_client = session.client('s3',
                           region_name=DO_SPACES_REGION,
                           aws_access_key_id=DO_SPACES_KEY,
                           aws_secret_access_key=DO_SPACES_SECRET,
                           endpoint_url=f'https://sfo2.digitaloceanspaces.com')


class CardProcessingError(Exception):
    def __init__(self, message, error_code):
        super().__init__(message)
        self.error_code = error_code


class CustomJSONEncoder(json.JSONEncoder):
    def encode(self, obj):
        def float_formatter(o):
            if isinstance(o, float):
                return format(o, ".5f")
            raise TypeError(repr(o) + " is not JSON serializable")

        return super(CustomJSONEncoder, self).encode(self, default=float_formatter)
    
def check_name_duplicates(cards_file):
    print("Checking for name duplicates in the entire 'cards.json' file...")
    with open(cards_file, "r", encoding="utf-8") as f:
        cards_data = json.load(f)

    # Count the occurrences of each card name using a Counter object
    name_counter = Counter(card.get("name") for card in cards_data)

    # Find the set of card names that occur more than once
    duplicates = set(name for name, count in name_counter.items() if count > 1)

    if len(duplicates) > 0:
        print("The following card names have duplicates:")
        for name in duplicates:
            print(f"- {name}")
    else:
        print("No name duplicates were found.")

def is_valid_filename(filename):
    # Modify this regular expression to match the correct file name format
    pattern = re.compile(r'\d+_\w+_\w+\.\w+')
    return pattern.match(filename) is not None

def rename_files(directory):
    for root, dirs, files in os.walk(directory):
        for file in files:
            file_path = os.path.join(root, file)
            file_name, file_ext = os.path.splitext(file)
            new_file_name = file_name.lower().replace(' ', '_')
            new_file_path = os.path.join(root, new_file_name + file_ext)
            os.rename(file_path, new_file_path)

def create_folder(folder_name, target_directory):
    print(f"Creating folder: {target_directory}/{folder_name}")
    s3_client.put_object(Bucket=DO_SPACES_BUCKET, Key=f"{target_directory}/{folder_name}/")
    print(f"Folder created: {target_directory}/{folder_name}")

def upload_file(directory, file_path, folder_name, target_directory):
    relative_path = Path(file_path).relative_to(directory).as_posix()
    if not is_valid_filename(relative_path):
        print(f"Warning: Invalid file name '{relative_path}'. Skipping file.")
        return None

    file_name, file_ext = os.path.splitext(relative_path)
    new_file_ext = file_ext if file_ext.lower() == ".gif" else ".jpg"  # Preserve .gif extension
    relative_path_new_ext = file_name + new_file_ext

    print(f"Uploading file: {file_path}")
    try:
        content_type = "image/gif" if file_ext.lower() == '.gif' else "image/jpeg"
        s3_client.upload_file(
            file_path,
            DO_SPACES_BUCKET,
            f"{target_directory}/{folder_name}/{relative_path_new_ext}",
            ExtraArgs={'ACL': 'public-read', 'ContentType': content_type}  # Add 'ContentType' parameter
        )
    except Exception as e:
        print(f"Error uploading file '{file_path}': {e}")
        raise CardProcessingError(f"Error uploading file '{file_path}'", 1) from e
    print(f"File uploaded: {file_path}")
    return relative_path



def upload_files(directory, folder_name, target_directory):
    print(f"Uploading files to: cards/{target_directory}/{folder_name}")
    valid_files = []
    all_files = [os.path.join(root, file) for root, dirs, files in os.walk(directory) for file in files]

    with ThreadPoolExecutor() as executor:
        futures = {executor.submit(upload_file, directory, file_path, folder_name, target_directory): file_path for file_path in all_files}
        for future in as_completed(futures):
            result = future.result()
            if result is not None:
                valid_files.append(result)

    print(f"Finished uploading {len(valid_files)} files to: cards/{target_directory}/{folder_name}")
    return valid_files

def update_json_files(directory, cards_file, collections_file, target_directory, valid_files):
    try:
        with open(cards_file, "r", encoding="utf-8") as f:
            cards_data = json.load(f)
        with open(collections_file, "r", encoding="utf-8") as f:
            collections_data = json.load(f)

        folder_name = os.path.basename(directory)

        next_id = max(card["id"] for card in cards_data if "id" in card) + 1 if cards_data else 0

        new_collection_added = False

        if not any(collection["id"] == folder_name for collection in collections_data):
            new_collection = {
                "id": folder_name,
                "name": folder_name.upper(),
                "origin": None,
                "aliases": [folder_name],
                "promo": False,
                "compressed": True,
                "rarity": -1,
                "tags": [target_directory],
            }
            new_collection_added = True
            collections_data.append(new_collection)
            print(f"Updating JSON files")

        for root, dirs, files in os.walk(directory):
            for file in files:
                if file not in valid_files:
                    continue
                file_name, file_ext = os.path.splitext(file)

                # Check if the file is a .gif
                is_animated = file_ext.lower() == ".gif"

                if file_ext.lower() in ACCEPTED_EXTENSIONS.union({".gif"}):  # Include .gif in accepted extensions
                    card_id = file_name.lower()
                    level = int(card_id.split("_")[0])
                    name_without_prefix = "_".join(card_id.split("_")[1:])
                    existing_card = next((card for card in cards_data if card["name"] == name_without_prefix and card["col"] == folder_name), None)
                    if existing_card:
                        print(f"Card with name '{name_without_prefix}' already exists in collection '{folder_name}'. Skipping file.")
                    else:
                        new_card = {
                            "name": name_without_prefix,
                            "level": level,
                            "animated": is_animated,  # Set animated field based on the file extension
                            "col": folder_name,
                            "id": next_id,
                            "tags": target_directory,
                            "added": datetime.now().isoformat(),
                        }
                        cards_data.append(new_card)
                        next_id += 1
                        print(f"Finished updating JSON files")

        with open(cards_file, "w", encoding="utf-8") as f:
            json.dump(cards_data, f, indent=2, ensure_ascii=False, cls=CustomJSONEncoder)
        with open(collections_file, "w", encoding="utf-8") as f:
            json.dump(collections_data, f, indent=2, ensure_ascii=False, cls=CustomJSONEncoder)
    except Exception as e:
        print(f"Error updating JSON files: {e}")
        raise CardProcessingError("Error updating JSON files", 2) from e
        
def process_directory(input_directory, cards_file, collections_file, target_directory, promo_prefix):
    try:
        folder_name = os.path.basename(input_directory)
        create_folder(folder_name, promo_prefix + target_directory)
        rename_files(input_directory)
        valid_files = upload_files(input_directory, folder_name, promo_prefix + target_directory)
        new_collection_added = update_json_files(input_directory, cards_file, collections_file, target_directory, valid_files)
        return valid_files, new_collection_added
    except CardProcessingError as e:
        print(f"Error in process: {e.error_code} - {e}")
        return [], False


    

# Add this new function to write the output file
def write_output_file(output_file, input_directory, valid_files, last_card_id, new_collection_added, cards_file, target_directory, error_messages):
    collection = os.path.basename(input_directory)

    with open(output_file, "a") as f:
        f.write(f"Group Type: {target_directory}\n")  # Add the group type
        f.write(f"Collection Name: {collection}\n")
        f.write(f"Number of Cards: {len(valid_files)}\n")
        f.write("Uploaded Files:\n")
        for file in valid_files:
            f.write(f"  {file}\n")

        first_card_id = None
        with open(cards_file, "r") as cf:  # Read the updated cards_data
            cards_data = json.load(cf)
            first_card_id = cards_data[-len(valid_files)]['id'] if cards_data and valid_files else None  # Get the first card ID from the cards_data
        f.write(f"First Card ID: {first_card_id}\n")
        f.write(f"Last Card ID: {last_card_id}\n")

        if error_messages:
            f.write("Errors:\n")
        for error_message in error_messages:
            f.write(f"  {error_message}\n")
        f.write("\n")

        if new_collection_added:
            f.write(f"New collection '{collection}' added.\n")
        else:
            f.write(f"Collection '{collection}' updated.\n")
        f.write("\n")

def main(input_directories, cards_file, collections_file, output_file):
    for target_directory in ["girlgroups", "boygroups", "promo_girlgroups", "promo_boygroups"]:
        promo_prefix = "promo/" if "promo" in target_directory else ""
        actual_target_directory = target_directory.replace("promo_", "")
        for input_directory in input_directories[target_directory]:
            error_messages = []  # Add this line to define the error_messages list
            try:
                valid_files, new_collection_added = process_directory(input_directory, cards_file, collections_file, actual_target_directory, promo_prefix)
            except Exception as e:
                error_message = f"Error in process for {input_directory}: {str(e)}"
                print(error_message)
                error_messages.append(error_message)  # Append the error message to the error_messages list
                continue  # Skip the current iteration and move to the next input_directory

            last_card_id = None
            with open(cards_file, "r") as f:
                cards_data = json.load(f)
                last_card_id = cards_data[-1]["id"] if cards_data else None

            # Call write_output_file() function for each input_directory, passing only the current input_directory
            write_output_file(output_file, input_directory, valid_files, last_card_id, new_collection_added, cards_file, target_directory, error_messages)  # Add the error_messages parameter


if __name__ == "__main__":
    import sys
    girlgroups_directory = sys.argv[1]
    boygroups_directory = sys.argv[2]
    cards_file = sys.argv[3]
    collections_file = sys.argv[4]
    output_file = sys.argv[5]
    promo_directory = sys.argv[6]  # Add a new command line argument for the promo directory

    input_directories = {
    "girlgroups": [os.path.join(girlgroups_directory, folder) for folder in os.listdir(girlgroups_directory) if os.path.isdir(os.path.join(girlgroups_directory, folder))],
    "boygroups": [os.path.join(boygroups_directory, folder) for folder in os.listdir(boygroups_directory) if os.path.isdir(os.path.join(boygroups_directory, folder))],
    "promo_girlgroups": [os.path.join(promo_directory, "girlgroups", folder) for folder in os.listdir(os.path.join(promo_directory, "girlgroups")) if os.path.isdir(os.path.join(promo_directory, "girlgroups", folder))],
    "promo_boygroups": [os.path.join(promo_directory, "boygroups", folder) for folder in os.listdir(os.path.join(promo_directory, "boygroups")) if os.path.isdir(os.path.join(promo_directory, "boygroups", folder))],
    }

    main(input_directories, cards_file, collections_file, output_file)


