#!/usr/bin/env python3

import sys
import os
import glob
import random


def shuffle_no_consecutive(lst):
    new_lst = []
    old_lst = random.sample(lst, len(lst))

    unqiue_lst = list(set(old_lst))[0:5]
    for unqiue in unqiue_lst:
        old_lst.remove(unqiue)
    old_lst = unqiue_lst + old_lst

    for oldi in range(len(old_lst)):
        e = old_lst[oldi]

        if len(new_lst) == 0:
            new_lst.insert(0, e)
        else:
            for newi in range(len(new_lst)):
                if newi == len(new_lst) - 1:
                    new_lst.insert(newi + 1, e)
                    break

                if newi == 0 and new_lst[newi] != e:
                    new_lst.insert(newi, e)
                    break

                if new_lst[newi] != e and new_lst[newi + 1] != e:
                    new_lst.insert(newi + 1, e)
                    break

    return new_lst


def main(types, video_dir, output_file):
    TYPES = types.split(",")
    videos = []
    for t in TYPES:
        for video in sorted(glob.glob(os.path.join(video_dir, f"*.{t}"))):
            filename = os.path.basename(video)

            parts = filename.split(".")
            if len(parts) > 1 and parts[0].isdigit():
                weight = int(parts[0])
            else:
                weight = 5

            for _ in range(weight):
                videos.append(video)

    videos = shuffle_no_consecutive(videos)
    with open(output_file, "w", encoding="utf-8") as f:
        for video in videos:
            f.write(f"file '{video}'\n")

    print(f"M3U file created: {output_file}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2], sys.argv[3])
