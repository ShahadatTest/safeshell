// Package rules contains one file per tool family (fs, disk, git, docker, k8s,
// cloud, db, pkg, net, proc, system). Each file registers its rules with classify.
//
// Adding support for a new tool means adding a rule file and corpus entries, nothing else.
package rules
