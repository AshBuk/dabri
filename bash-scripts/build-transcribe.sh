#!/bin/sh
# Build transcribe.cpp into one private shared library.
#
# transcribe.cpp vendors its own ggml, a different version from the one in whisper.cpp.
# The static ggml is linked in with its symbols hidden, so the library exports only
# transcribe_* and both ggml copies can live in one process.
#
# x86-64 builds target AVX2, and ModelDef.CPUSupported keeps these models off older CPUs.
# The flags are spelled out because ggml turns them all off when SOURCE_DATE_EPOCH is set.
#
# Usage: build-transcribe.sh <transcribe.cpp source dir> <output dir>
set -eu
src=$1
out=$2

cmake -S "$src" -B "$src/build" -DCMAKE_BUILD_TYPE=Release -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    -DTRANSCRIBE_BUILD_TESTS=OFF -DTRANSCRIBE_BUILD_EXAMPLES=OFF -DTRANSCRIBE_INSTALL=OFF \
    -DTRANSCRIBE_VULKAN=ON -DBLA_VENDOR=OpenBLAS \
    -DGGML_NATIVE=OFF -DGGML_SSE42=ON -DGGML_AVX=ON -DGGML_AVX2=ON -DGGML_FMA=ON -DGGML_F16C=ON -DGGML_BMI2=ON -DGGML_AVX512=OFF
# A bare --parallel means unlimited jobs with the Makefiles generator and runs the machine out of memory
cmake --build "$src/build" --parallel "${CMAKE_BUILD_PARALLEL_LEVEL:-$(nproc)}"

mkdir -p "$out"
# shellcheck disable=SC2046,SC2086 # the archive list and LDFLAGS are meant to split into words
${CXX:-c++} -shared -o "$out/libdabri-transcribe.so" -Wl,-soname,libdabri-transcribe.so ${LDFLAGS:-} \
    -Wl,--whole-archive "$src/build/src/libtranscribe.a" -Wl,--no-whole-archive \
    -Wl,--start-group $(find "$src/build/ggml" -name 'libggml*.a') -Wl,--end-group \
    -Wl,--exclude-libs,libggml.a:libggml-base.a:libggml-cpu.a:libggml-vulkan.a \
    -Wl,-z,defs -lopenblas -lvulkan -lpthread -ldl -lm
# Without CBLAS the decoder silently falls back to scalar loops, about 40 times slower
nm -D --undefined-only "$out/libdabri-transcribe.so" | grep -q cblas_sgemm || {
    echo "ERROR: transcribe.cpp was built without CBLAS (install OpenBLAS headers)" >&2
    exit 1
}
cp "$src/include/transcribe.h" "$out/"
