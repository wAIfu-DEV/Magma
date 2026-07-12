#define WIN32_LEAN_AND_MEAN
#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <windows.h>

/*
 * Compile the allocation benchmark with -fno-builtin-malloc and
 * -fno-builtin-free. Otherwise an optimizing compiler may legally remove the
 * paired calls because the allocated pointer does not escape.
 */

enum {
    MEMORY_PASSES = 8,
};

static const uint64_t ELEMENT_COUNT = UINT64_C(8388608);
static const uint64_t MASK = UINT64_C(8388607);
static const uint64_t CPU_ITERATIONS = UINT64_C(100000000);
static const uint64_t RANDOM_ITERATIONS = UINT64_C(25000000);
static const uint64_t DISPATCH_ITERATIONS = UINT64_C(100000000);
static const uint64_t ALLOCATION_COUNT = UINT64_C(1000000);

typedef uint64_t (*StepFn)(void *, uint64_t);

typedef struct Stepper {
    void *state;
    StepFn step_fn;
} Stepper;

static int64_t ticks(void) {
    LARGE_INTEGER value;
    QueryPerformanceCounter(&value);
    return value.QuadPart;
}

static double seconds_between(int64_t start, int64_t finish, int64_t frequency) {
    return (double)(finish - start) / (double)frequency;
}

static uint64_t mix64(uint64_t value) {
    uint64_t x = value;
    x = (x ^ (x >> 30)) * UINT64_C(0xBF58476D1CE4E5B9);
    x = (x ^ (x >> 27)) * UINT64_C(0x94D049BB133111EB);
    return x ^ (x >> 31);
}

static uint64_t cpu_kernel(uint64_t iterations, uint64_t seed) {
    uint64_t x = seed;
    uint64_t sum = 0;

    for (uint64_t i = 0; i < iterations; ++i) {
        x += UINT64_C(0x9E3779B97F4A7C15);
        x = mix64(x);
        sum ^= x;
    }
    return sum;
}

static uint64_t fill_kernel(uint64_t *data, uint64_t count, uint64_t seed) {
    uint64_t x = seed;
    uint64_t sum = 0;

    for (uint64_t i = 0; i < count; ++i) {
        x += UINT64_C(0x9E3779B97F4A7C15);
        x = mix64(x);
        data[i] = x;
        sum += x;
    }
    return sum;
}

static uint64_t sequential_memory_kernel(uint64_t *data, uint64_t count,
                                         uint64_t passes) {
    uint64_t checksum = 0;

    for (uint64_t pass = 0; pass < passes; ++pass) {
        for (uint64_t i = 0; i < count; ++i) {
            uint64_t value = data[i];
            value = (value * 33) ^ (value >> 11) ^ i;
            data[i] = value;
            checksum += value;
        }
    }
    return checksum;
}

static uint64_t random_access_kernel(const uint64_t *data, uint64_t iterations,
                                     uint64_t seed) {
    uint64_t index = seed & MASK;
    uint64_t checksum = seed;

    for (uint64_t i = 0; i < iterations; ++i) {
        index = (data[index] ^ checksum) & MASK;
        checksum += data[index];
    }
    return checksum ^ index;
}

static uint64_t dispatch_step(void *state, uint64_t value) {
    uint64_t *s = (uint64_t *)state;
    uint64_t next = (*s * UINT64_C(1664525)) + UINT64_C(1013904223) + value;
    *s = next;
    return next ^ (next >> 17);
}

static uint64_t dispatch_kernel(Stepper *stepper, uint64_t iterations) {
    uint64_t checksum = 0;

    for (uint64_t i = 0; i < iterations; ++i) {
        checksum += stepper->step_fn(stepper->state, i);
    }
    return checksum;
}

static uint64_t allocation_kernel(uint64_t count) {
    uint64_t checksum = 0;

    for (uint64_t i = 0; i < count; ++i) {
        uint64_t *block = (uint64_t *)malloc(64);
        if (block == NULL) {
            fputs("malloc failed\n", stderr);
            exit(1);
        }
        block[0] = i;
        block[7] = i ^ UINT64_C(0xA5A5A5A5);
        checksum += block[0] + block[7];
        free(block);
    }
    return checksum;
}

static void print_result(const char *name, double elapsed, uint64_t checksum) {
    printf("%s: %.6f s  checksum=%" PRIu64 "\n", name, elapsed, checksum);
}

int main(int argc, char **argv) {
    (void)argv;

    LARGE_INTEGER frequency_value;
    if (!QueryPerformanceFrequency(&frequency_value)) {
        fputs("QueryPerformanceFrequency failed\n", stderr);
        return 1;
    }
    const int64_t frequency = frequency_value.QuadPart;
    const uint64_t seed = UINT64_C(0x123456789ABCDEF0) + (uint64_t)argc;
    uint64_t *data = (uint64_t *)malloc(ELEMENT_COUNT * sizeof(*data));
    if (data == NULL) {
        fputs("64 MiB allocation failed\n", stderr);
        return 1;
    }

    puts("C performance benchmark (release builds only)");
    puts("Dataset: 64 MiB; times exclude allocation/setup unless named");

    int64_t start = ticks();
    uint64_t checksum = cpu_kernel(CPU_ITERATIONS, seed);
    int64_t finish = ticks();
    print_result("scalar integer", seconds_between(start, finish, frequency), checksum);

    start = ticks();
    checksum = fill_kernel(data, ELEMENT_COUNT, checksum);
    finish = ticks();
    print_result("fill 64 MiB", seconds_between(start, finish, frequency), checksum);

    start = ticks();
    checksum = sequential_memory_kernel(data, ELEMENT_COUNT, MEMORY_PASSES);
    finish = ticks();
    print_result("sequential memory", seconds_between(start, finish, frequency), checksum);

    start = ticks();
    checksum = random_access_kernel(data, RANDOM_ITERATIONS, checksum);
    finish = ticks();
    print_result("random memory", seconds_between(start, finish, frequency), checksum);

    uint64_t dispatch_state = checksum;
    Stepper stepper = {&dispatch_state, dispatch_step};
    start = ticks();
    checksum = dispatch_kernel(&stepper, DISPATCH_ITERATIONS);
    finish = ticks();
    print_result("function dispatch", seconds_between(start, finish, frequency), checksum);

    start = ticks();
    checksum = allocation_kernel(ALLOCATION_COUNT);
    finish = ticks();
    print_result("allocation churn", seconds_between(start, finish, frequency), checksum);

    free(data);
    puts("Done. Compare averages from repeated runs on an idle machine.");
    return 0;
}
