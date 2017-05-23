package gocb

import (
	"encoding/json"
	"gopkg.in/couchbase/gocbcore.v2"
	"log"
)

type subDocResult struct {
	path string
	data []byte
	err  error
}

// Represents multiple chunks of a full Document.
type DocumentFragment struct {
	cas      Cas
	mt       MutationToken
	contents []subDocResult
	pathMap  map[string]int
}

// Returns the Cas of the Document
func (frag *DocumentFragment) Cas() Cas {
	return frag.cas
}

// Returns the MutationToken for the change represented by this DocumentFragment.
func (frag *DocumentFragment) MutationToken() MutationToken {
	return frag.mt
}

// Retrieve the value of the operation by its index. The index is the position of
// the operation as it was added to the builder.
func (frag *DocumentFragment) ContentByIndex(idx int, valuePtr interface{}) error {
	res := frag.contents[idx]
	if res.err != nil {
		return res.err
	}
	if valuePtr == nil {
		return nil
	}
	return json.Unmarshal(res.data, valuePtr)
}

// Retrieve the value of the operation by its path. The path is the path provided
// to the operation
func (frag *DocumentFragment) Content(path string, valuePtr interface{}) error {
	if frag.pathMap == nil {
		frag.pathMap = make(map[string]int)
		for i, v := range frag.contents {
			frag.pathMap[v.path] = i
		}
	}
	return frag.ContentByIndex(frag.pathMap[path], valuePtr)
}

// Checks whether the indicated path exists in this DocumentFragment and no
// errors were returned from the server.
func (frag *DocumentFragment) Exists(path string) bool {
	err := frag.Content(path, nil)
	return err == nil
}

// Builder used to create a set of sub-document lookup operations.
type LookupInBuilder struct {
	bucket *Bucket
	name   string
	ops    []gocbcore.SubDocOp
}

// Executes this set of lookup operations on the bucket.
func (set *LookupInBuilder) Execute() (*DocumentFragment, error) {
	return set.bucket.lookupIn(set)
}

// Indicate a path to be retrieved from the document.  The value of the path
// can later be retrieved (after .Execute()) using the Content or ContentByIndex
// method. The path syntax follows N1QL's path syntax (e.g. `foo.bar.baz`).
func (set *LookupInBuilder) Get(path string) *LookupInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpGet,
		Path: path,
	}
	set.ops = append(set.ops, op)
	return set
}

// Similar to Get(), but does not actually retrieve the value from the server.
// This may save bandwidth if you only need to check for the existence of a
// path (without caring for its content). You can check the status of this
// operation by using .Content (and ignoring the value) or .Exists()
func (set *LookupInBuilder) Exists(path string) *LookupInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpExists,
		Path: path,
	}
	set.ops = append(set.ops, op)
	return set
}

func (b *Bucket) lookupIn(set *LookupInBuilder) (resOut *DocumentFragment, errOut error) {
	signal := make(chan bool, 1)
	op, err := b.client.SubDocLookup([]byte(set.name), set.ops,
		func(results []gocbcore.SubDocResult, cas gocbcore.Cas, err error) {
			errOut = err

			{
				resSet := &DocumentFragment{}
				resSet.contents = make([]subDocResult, len(results))

				for i := range results {
					resSet.contents[i].path = set.ops[i].Path
					resSet.contents[i].err = results[i].Err
					if results[i].Value != nil {
						resSet.contents[i].data = append([]byte(nil), results[i].Value...)
					}
				}

				resOut = resSet
			}

			signal <- true
		})
	if err != nil {
		return nil, err
	}

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return nil, ErrTimeout
	}
}

// Creates a sub-document lookup operation builder.
func (b *Bucket) LookupIn(key string) *LookupInBuilder {
	return &LookupInBuilder{
		bucket: b,
		name:   key,
	}
}

// Builder used to create a set of sub-document mutation operations.
type MutateInBuilder struct {
	bucket *Bucket
	name   string
	cas    gocbcore.Cas
	expiry uint32
	ops    []gocbcore.SubDocOp
}

// Executes this set of mutation operations on the bucket.
func (set *MutateInBuilder) Execute() (*DocumentFragment, error) {
	return set.bucket.mutateIn(set)
}

// Adds an insert operation to this mutation operation set.
func (set *MutateInBuilder) Insert(path string, value interface{}, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpDictAdd,
		Path: path,
	}
	op.Value, _ = json.Marshal(value)
	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

// Adds an upsert operation to this mutation operation set.
func (set *MutateInBuilder) Upsert(path string, value interface{}, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpDictSet,
		Path: path,
	}
	op.Value, _ = json.Marshal(value)
	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

// Adds an replace operation to this mutation operation set.
func (set *MutateInBuilder) Replace(path string, value interface{}) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpReplace,
		Path: path,
	}
	op.Value, _ = json.Marshal(value)
	set.ops = append(set.ops, op)
	return set
}

func (set *MutateInBuilder) marshalArrayMulti(in interface{}) (out []byte) {
	out, err := json.Marshal(in)
	if err != nil {
		log.Panic(err)
	}

	// Assert first character is a '['
	if len(out) < 2 || out[0] != '[' {
		log.Panic("Not a JSON array")
	}

	out = out[1 : len(out)-1]
	return
}

// Adds an remove operation to this mutation operation set.
func (set *MutateInBuilder) Remove(path string) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpDelete,
		Path: path,
	}
	set.ops = append(set.ops, op)
	return set
}

// Adds an element to the beginning (i.e. left) of an array
func (set *MutateInBuilder) ArrayPrepend(path string, value interface{}, createParents bool) *MutateInBuilder {
	jsonVal, _ := json.Marshal(value)

	return set.arrayPrependValue(path, jsonVal, createParents)
}

func (set *MutateInBuilder) arrayPrependValue(path string, bytes []byte, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpArrayPushFirst,
		Path: path,
	}

	op.Value = bytes

	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

// Adds an element to the end (i.e. right) of an array
func (set *MutateInBuilder) ArrayAppend(path string, value interface{}, createParents bool) *MutateInBuilder {
	jsonVal, _ := json.Marshal(value)

	return set.arrayAppendValue(path, jsonVal, createParents)
}

func (set *MutateInBuilder) arrayAppendValue(path string, bytes []byte, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpArrayPushLast,
		Path: path,
	}

	op.Value = bytes

	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

// Inserts an element at a given position within an array. The position should be
// specified as part of the path, e.g. path.to.array[3]
func (set *MutateInBuilder) ArrayInsert(path string, value interface{}) *MutateInBuilder {
	jsonVal, _ := json.Marshal(value)

	return set.arrayInsertValue(path, jsonVal)
}

func (set *MutateInBuilder) arrayInsertValue(path string, bytes []byte) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpArrayInsert,
		Path: path,
	}

	op.Value = bytes
	set.ops = append(set.ops, op)
	return set
}

// Adds multiple values as elements to an array.
// `values` must be an array type
// ArrayAppendMulti("path", []int{1,2,3,4}, true) =>
//   "path" [..., 1,2,3,4]
//
// This is a more efficient version (at both the network and server levels)
// of doing
// ArrayAppend("path", 1, true).ArrayAppend("path", 2, true).ArrayAppend("path", 3, true)
//
// See ArrayAppend() for more information
func (set *MutateInBuilder) ArrayAppendMulti(path string, values interface{}, createParents bool) *MutateInBuilder {
	return set.arrayAppendValue(path, set.marshalArrayMulti(values), createParents)
}

// Adds multiple values at the beginning of an array.
// See ArrayAppendMulti for more information about multiple element operations
// and ArrayPrepend for the semantics of this operation
func (set *MutateInBuilder) ArrayPrependMulti(path string, values interface{}, createParents bool) *MutateInBuilder {
	return set.arrayPrependValue(path, set.marshalArrayMulti(values), createParents)
}

// Inserts multiple elements at a specified position within the
// array. See ArrayAppendMulti for more information about multiple element
// operations, and ArrayInsert for more information about array insertion operations
func (set *MutateInBuilder) ArrayInsertMulti(path string, values interface{}) *MutateInBuilder {
	return set.arrayInsertValue(path, set.marshalArrayMulti(values))
}

// Adds an dictionary add unique operation to this mutation operation set.
func (set *MutateInBuilder) ArrayAddUnique(path string, value interface{}, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpArrayAddUnique,
		Path: path,
	}
	op.Value, _ = json.Marshal(value)
	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

// Adds an counter operation to this mutation operation set.
func (set *MutateInBuilder) Counter(path string, delta int64, createParents bool) *MutateInBuilder {
	op := gocbcore.SubDocOp{
		Op:   gocbcore.SubDocOpCounter,
		Path: path,
	}
	op.Value, _ = json.Marshal(delta)
	if createParents {
		op.Flags |= gocbcore.SubDocFlagMkDirP
	}
	set.ops = append(set.ops, op)
	return set
}

func (b *Bucket) mutateIn(set *MutateInBuilder) (resOut *DocumentFragment, errOut error) {
	signal := make(chan bool, 1)
	op, err := b.client.SubDocMutate([]byte(set.name), set.ops, set.cas, set.expiry,
		func(results []gocbcore.SubDocResult, cas gocbcore.Cas, mt gocbcore.MutationToken, err error) {
			errOut = err
			if errOut == nil {
				resSet := &DocumentFragment{
					cas: Cas(cas),
					mt:  MutationToken{mt, b},
				}
				resSet.contents = make([]subDocResult, len(results))

				for i := range results {
					resSet.contents[i].path = set.ops[i].Path
					resSet.contents[i].err = results[i].Err
					if results[i].Value != nil {
						resSet.contents[i].data = append([]byte(nil), results[i].Value...)
					}
				}

				resOut = resSet
			}
			signal <- true
		})
	if err != nil {
		return nil, err
	}

	timeoutTmr := gocbcore.AcquireTimer(b.opTimeout)
	select {
	case <-signal:
		gocbcore.ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		gocbcore.ReleaseTimer(timeoutTmr, true)
		if !op.Cancel() {
			<-signal
			return
		}
		return nil, ErrTimeout
	}
}

// Creates a sub-document mutation operation builder.
func (b *Bucket) MutateIn(key string, cas Cas, expiry uint32) *MutateInBuilder {
	return &MutateInBuilder{
		bucket: b,
		name:   key,
		cas:    gocbcore.Cas(cas),
		expiry: expiry,
	}
}
