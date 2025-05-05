from setuptools import setup, find_packages

setup(
    name="mindweaver",
    version="0.1.0",
    packages=find_packages(where="scripts"),
    package_dir={"": "scripts"},
    install_requires=[
        "pyqt6",
        "python-dotenv",
    ],
    entry_points={
        "console_scripts": [
            "mindweaver=scripts.loom.main:main"
        ]
    },
)
