import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import sys
import os

def line_plot(name, xdata, ydata, xlabel, ylabel, figsize):
    fig = plt.figure(figsize=figsize)
    ax = fig.add_subplot(111)
    ax.plot(xdata, ydata, marker="o")
    ax.set_xlabel(xlabel)
    ax.set_ylabel(ylabel)
    plt.savefig(name + ".png", bbox_inches="tight")

def main():
    xdata = [20, 72, 137, 302, 658, 1152]
    ydata = [123, 153, 138, 173, 233, 259]
    line_plot("Model_Inference", xdata, ydata, "Number of Events", "Runtime (in ms)", (8,4))

if __name__ == '__main__':
    main()
