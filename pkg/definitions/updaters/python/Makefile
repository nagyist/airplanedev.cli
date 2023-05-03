lint:
	poetry run mypy ./src
	poetry run pylint ./src
	poetry run isort ./src --check
	poetry run black ./src --check

lint-fix:
	poetry run isort ./src
	poetry run black ./src

build:
	@# Set --venv since it makes pex files run ~1/3 faster (200ms). See:
	@# https://github.com/pantsbuild/pex/issues/839#issuecomment-1087746084
	@# Set `--no-compile` so that the generated pex file is reproducible. This allows CI to
	@# confirm that the checked-in file has been generated from the latest source.
	@# Set `--sh-boot` to improve the performance on subsequent runs (see pex --help).
	@# Generate a requirements.txt in order to pass our exact dependency versions into pex.
	@# https://zameermanji.com/blog/2021/6/25/packaging-multi-platform-python-applications/
	@# https://pex.readthedocs.io/en/latest/buildingpex.html?highlight=wheel#tailoring-requirement-resolution
	@poetry export --without-hashes --without=dev --format=requirements.txt > requirements.txt
	@cut -d ";" -f1 requirements.txt > requirements-no-markers.txt
	@rm -rf wheels && \
		pip download \
			-r requirements-no-markers.txt \
			--dest ./wheels \
			--only-binary :all: \
			--platform=any \
			--implementation=py \
			--disable-pip-version-check
	@# If this errors with "The `python3.8' command exists in these Python versions:",
	@# then run "pyenv global 3.11 3.10 3.9 3.8". See: https://github.com/pyenv/pyenv/issues/34#issuecomment-420679210
	time poetry run pex \
		-vv \
		--sources-directory ./src \
		--exe ./src/main.py \
		--requirement requirements.txt \
		--no-compile \
		--sh-boot \
		--venv \
		-f ./wheels \
		--no-index \
		--python python3.8 \
		--python python3.9 \
		--python python3.10 \
		--python python3.11 \
		--output-file ./updater.pex
	@echo "Size: `du -h ./updater.pex | cut -f1`"
	@rm -rf wheels requirements*.txt

.PHONY: lint lint-fix build
