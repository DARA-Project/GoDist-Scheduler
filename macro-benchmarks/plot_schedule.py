import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import sys
import os

def get_column(filename, col_name):
    df = pd.read_csv(filename)
    return df[col_name]

def get_value(filename, name):
    column = get_column(filename, name)
    return np.mean(column)

def plot_vertical_bars(name, data, xlabels, legend, ylabel,figsize, xlabel=""):
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
        ax.set_xticks(indices + width * (len(data)-1)/2)
        ax.set_xticklabels(xlabels)
        if xlabel != "":
            ax.set_xlabel(xlabel)
        ax.legend(rects, legend, loc='lower left', bbox_to_anchor=(0.0,1.01), frameon=False, ncol=len(data))
        #ax.legend(rects, legend, ncol=len(data))
        plt.savefig(name + ".png", bbox_inches="tight")

def line_plot(name, xdata, ydata, xlabel, ylabel, figsize):
    fig = plt.figure(figsize=figsize)
    ax = fig.add_subplot(111)
    ax.plot(xdata, ydata, marker="o")
    ax.set_xlabel(xlabel)
    ax.set_ylabel(ylabel)
    plt.savefig(name + ".png", bbox_inches="tight")

def main():
    if len(sys.argv) < 3:
        print("Usage: python plot_schedule.py [option] <events.csv> <rest_of_args>")
        sys.exit(1) 
    option = sys.argv[1]
    print(option)
    if option == "mve":
        events_file = sys.argv[2]
        memory_file = sys.argv[3]
        events = get_column(events_file, "Events")
        memory = get_column(memory_file, "Memory") / 1024
        line_plot("memory_vs_events",events, memory, "Number of Events", "Schedule Size (in KB)", (8,4))
    elif option == "time":
        events_file = sys.argv[2]
        go_vals = []
        record_vals = []
        replay_vals = []
        names = []
        for f in sys.argv[2:]:
            name = os.path.splitext(os.path.basename(f))[0]
            names += [name.split('-')[1]]
            go_val = get_value(f, "Normal")
            record_val = get_value(f, "Record")
            replay_val = get_value(f, "Replay")
            go_vals += [go_val/go_val]
            record_vals += [record_val/go_val]
            replay_vals += [replay_val/go_val]
        data = []
        data += [go_vals]
        data += [record_vals]
        data += [replay_vals]
        plot_vertical_bars("Replay_ffwd", data, names, ['Go v1.10.4', 'Record', 'Replay'], 'Slowdown factor', (8,4), "Number of iterations for SimpleFileReadIterative")
    elif option == "sched":
        events_file = sys.argv[2]
        go_vals = []
        record_vals = []
        replay_vals = []
        names = []
        for f in sys.argv[2:]:
            name = os.path.splitext(os.path.basename(f))[0]
            names += [name.split('-')[1]]
            go_val = get_value(f, "Normal")
            record_val = get_value(f, "Record")
            replay_val = get_value(f, "Replay")
            go_vals += [go_val/go_val]
            record_vals += [record_val/go_val]
            replay_vals += [replay_val/go_val]
        data = []
        data += [go_vals]
        data += [record_vals]
        data += [replay_vals]
        plot_vertical_bars("Sched_Replay", data, names, ['Go v1.10.4', 'Record', 'Replay'], 'Slowdown factor', (8,4), "Number of iterations for sharedIntegerChannelIterative")
    else :
        print("Invalid option selected")
        sys.exit(1)

if __name__ == '__main__':
    main()
