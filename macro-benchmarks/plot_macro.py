import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import sys
import os

def plot_vertical_bars(name, data, xlabels, legend, ylabel,figsize):
    N = len(data[0])
    indices = np.arange(N)
    patterns = ('//', '\\\\', 'o', '+', 'x', '*', '-', 'O', '.')
    # width = 1 / len(data)
    # This is the fitting width. Right now, lets just hardcode it
    if N == 1:
        new_indices = np.arange(len(legend))
        fig = plt.figure(figsize=figsize)
        ax = fig.add_subplot(111)
        i = 0
        rects = ()
        for d in data:
            rect = ax.bar([new_indices[i]],d, width=0.5, hatch=patterns[i]) 
            rects = rects + (rect[0],)
            i += 1

        ax.set_ylabel(ylabel)
        ax.set_xticks([])
        ax.set_xticklabels([])
        ax.set_xlabel(xlabels[0])
        ax.set_xlim(new_indices[0]-1, new_indices[len(legend)-1]+1)
        ax.legend(rects, legend, ncol=len(data))
        plt.savefig(name + ".png", bbox_inches="tight")
    if N >= 2:
        width = 0.15
        fig = plt.figure(figsize=figsize)
        ax = fig.add_subplot(111)

        i = 0
        rects = ()
        for d in data:
            rect = ax.bar(indices + width * i, d, width, hatch=patterns[i])
            rects = rects + (rect[0],)
            i += 1

        ax.set_ylabel(ylabel)
        ax.set_xlabel("Number of scheduled actions in the application")
        ax.set_xticks(indices + width * (len(data)-1)/2)
        ax.set_xticklabels(xlabels)
        #ax.legend(rects, legend, loc='lower left', bbox_to_anchor=(0.0,1.01), frameon=False, ncol=int(len(data)/2))
        ax.legend(rects, legend, ncol=len(data))
        plt.savefig(name + ".png", bbox_inches="tight")

def get_value(df, name):
    runs = pd.to_numeric(df[name])
    print(np.mean(runs))
    return np.mean(runs)

def main():
    if len(sys.argv) < 2:
        print("Usage: python plot_macro.py <list_of_benchmark_files>")
        sys.exit(1)
    files = sys.argv[1:]
    normal = []
    record = []
    replay = []
    names = []
    for f in files:
        print("Processing",f)
        name = os.path.splitext(os.path.basename(f))[0]
        df = pd.read_csv(f)
        normal_val = get_value(df, "Normal")
        record_val = get_value(df, "Record")
        replay_val = get_value(df, "Replay")
        normal += [normal_val/normal_val]
        record += [record_val/normal_val]
        replay += [replay_val/normal_val]
    data = []
    data += [normal]
    data += [record]
    data += [replay]
    df = pd.read_csv('events.csv')
    plot_vertical_bars("Macro_Bench", data, names, ['Go v1.10.4', 'Record', 'Schedule'], 'Slowdown factor', (8,4)) 

if __name__ == '__main__':
    main()
