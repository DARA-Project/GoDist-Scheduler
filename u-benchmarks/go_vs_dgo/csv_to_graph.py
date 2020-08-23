import matplotlib.pyplot as plt
import numpy as np
import sys
import csv

def get_data(filename):
    data = []
    with open(filename) as inf:
        reader = csv.DictReader(inf)
        for row in reader:
            data += [row]
    return data

def plot_vertical_bars(dataset_name, orderings, labels, legend):
    N = len(orderings[0])
    indices = np.arange(N)
    # width = 1 / len(orderings)
    # This is the fitting width. Right now, lets just hardcode it
    width = 0.15

    fig = plt.figure(figsize=(8,4))
    ax = fig.add_subplot(111)

    i = 0
    rects = ()
    for o in orderings:
        rect = ax.bar(indices + width * i, o, width)
        rects = rects + (rect[0],)
        i += 1

    ax.set_ylabel('Runtime (in ns)')
    ax.set_xticks(indices + width * (len(orderings)-1)/2)
    ax.set_xticklabels(labels, rotation=45)
    ax.legend(rects, legend, loc='lower left', bbox_to_anchor=(0.0,1.01), frameon=False, ncol=int(len(orderings)/2))
    plt.savefig(dataset_name + ".png", bbox_inches="tight")


def parse_data(dgo_data, go1_10_data):
    print("Name,Relative_Slow_Down,Percentage_Slow_Down")
    graphable_data_dgo = []
    graphable_data_go1_10 = []
    names = []
    for dgo, go1_10 in zip(dgo_data, go1_10_data):
        if dgo['name'] != go1_10['name']:
            print("The benchmarks should be in order")
            sys.exit(1)
        rel_sd = float(dgo['latency_ns'])/float(go1_10['latency_ns'])
        per_sd = (float(dgo['latency_ns']) - float(go1_10['latency_ns']))/float(go1_10['latency_ns'])
        print(dgo['name'], ",", rel_sd, ",", per_sd * 100)
        names += [dgo['name']]
        graphable_data_dgo += [float(dgo['latency_ns'])]
        graphable_data_go1_10 += [float(go1_10['latency_ns'])]
    return names, graphable_data_dgo, graphable_data_go1_10 

def main():
    if len(sys.argv) < 3:
        print("Usage: python csv_to_graph.py <dgo_file> <go1_10_file>")
        sys.exit(1)
    dgo_data = get_data(sys.argv[1])
    go1_10_data = get_data(sys.argv[2])
    names, graphable_data_dgo, graphable_data_go1_10 = parse_data(dgo_data, go1_10_data)
    length = 5
    legend = ['dgo', 'go 1.10']
    for i in range(int(len(names)/length)):
        startIndex = i * length
        endIndex = startIndex + length
        if i == int(len(names)/length) - 1:
            endIndex = len(names)
        data1 = graphable_data_dgo[startIndex:endIndex]
        data2 = graphable_data_go1_10[startIndex:endIndex]
        data = [data1, data2]
        plot_vertical_bars(str(i), data, names[startIndex:endIndex], legend)

if __name__ == '__main__':
    main()
