import os
import argparse
from datetime import datetime

EXCLUDE_DIRS = {'.git', '.github', '.idea', 'node_modules'}
EXCLUDE_FILES = {'scf.zip','server.db','zoneinfo.zip','.DS_Store','bash-obfuscate'}

def count_lines_and_words_in_file(filepath):
    lines = 0
    words = 0
    try:
        with open(filepath, 'r', encoding='utf-8') as file:
            for line in file:
                lines += 1
                words += len(line.split())
    except Exception as e:
        print(f"Error reading file {filepath}: {e}")
    return lines, words

def count_lines_and_words_in_directory(directory):
    total_lines = 0
    total_words = 0
    for root, dirs, files in os.walk(directory):
        # 排除指定的文件夹
        dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
        for file in files:
            if file in EXCLUDE_FILES:
                continue
            filepath = os.path.join(root, file)
            lines, words = count_lines_and_words_in_file(filepath)
            total_lines += lines
            total_words += words
    return total_lines, total_words

def main():
    parser = argparse.ArgumentParser(description="Count lines and words in a directory of files.")
    parser.add_argument('directory', type=str, help='Path to the directory')
    args = parser.parse_args()

    directory = args.directory

    # 获取当前脚本文件名，并添加到排除文件列表中
    current_script_name = os.path.basename(__file__)
    EXCLUDE_FILES.add(current_script_name)

    lines, words = count_lines_and_words_in_directory(directory)
    print(f'\n\n\n\n{'当前时间':<13}: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}')
    print(f'{'当前代码总行数':<10}: {lines} 行')
    print(f'{'代码量':<14}: {words} 个字符\n\n\n\n')

if __name__ == "__main__":
    main()
