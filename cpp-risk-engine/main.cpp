#include <iostream>
#include <string>
#include <cmath>

using namespace std;

// Decides risk level based on how full a zone is (as a percentage)
string getRiskLevel(int currentCount, int capacity) {
    double fillPercent = (static_cast<double>(currentCount) / capacity) * 100.0;

    if (fillPercent >= 90.0) {
        return "Danger";
    } else if (fillPercent >= 70.0) {
        return "Caution";
    } else {
        return "Safe";
    }
}

// Predicts minutes until a zone becomes dangerous (90% full),
// based on how fast the count is currently rising
int predictMinutesToDanger(int currentCount, int previousCount, int capacity, int minutesElapsed) {
    int peopleIncrease = currentCount - previousCount;

    // If crowd isn't growing (or is shrinking), there's no danger coming
    if (peopleIncrease <= 0 || minutesElapsed <= 0) {
        return -1; // -1 means "not applicable / not rising"
    }

    double growthRatePerMinute = static_cast<double>(peopleIncrease) / minutesElapsed;

    int dangerThreshold = static_cast<int>(capacity * 0.9); // 90% = danger point
    int peopleUntilDanger = dangerThreshold - currentCount;

    if (peopleUntilDanger <= 0) {
        return 0; // Already at/past danger point
    }

    double minutesUntilDanger = peopleUntilDanger / growthRatePerMinute;
    return static_cast<int>(minutesUntilDanger);
}

int main(int argc, char* argv[]) {
    // Expects: current_count previous_count capacity minutes_elapsed
    if (argc != 5) {
        cout << "Usage: risk_engine <current_count> <previous_count> <capacity> <minutes_elapsed>" << endl;
        return 1;
    }

    int currentCount = stoi(argv[1]);
    int previousCount = stoi(argv[2]);
    int capacity = stoi(argv[3]);
    int minutesElapsed = stoi(argv[4]);

    string riskLevel = getRiskLevel(currentCount, capacity);
    int etaDanger = predictMinutesToDanger(currentCount, previousCount, capacity, minutesElapsed);

    // Print output as: RiskLevel,EtaMinutes  (easy for Go to read later)
    cout << riskLevel << "," << etaDanger << endl;

    return 0;
}