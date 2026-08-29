package bigmem
import (
 "encoding/json"
 "errors"
 "testing"
)
func TestDeferredFKMissing(t *testing.T){
 s:=openTestStore(t)
 obs:=map[string]any{"id":"obs-deferred-1","title":"deferred","content":"c","session_id":"missing-sess","project":"proj-deferred"}
 pl,_:=json.Marshal(obs)
 m:=SyncMutation{Project:"proj-deferred",Entity:"observation",EntityKey:"obs-deferred-1",Payload:pl}
 if err:=s.ApplyPulledMutation(m);!errors.Is(err,ErrFKMissing){t.Fatalf("expected ErrFKMissing got %v",err)}
 var cnt,att int
 var sid string
 _=s.db.QueryRow(`SELECT sync_id,attempts FROM sync_apply_deferred WHERE entity_key=?`,"obs-deferred-1").Scan(&sid,&att)
 _=s.db.QueryRow(`SELECT COUNT(*) FROM sync_apply_deferred`).Scan(&cnt)
 if cnt!=1||att!=1{t.Fatalf("cnt=%d att=%d sid=%q",cnt,att,sid)}
 if sid!=pulledSessionDeadLetterSyncID("proj-deferred","obs-deferred-1",pl){t.Fatalf("sid %q want %q",sid,pulledSessionDeadLetterSyncID("proj-deferred","obs-deferred-1",pl))}
}
func TestDeferredDeadLetterHash(t *testing.T){
 s:=openTestStore(t)
 pl,_:=json.Marshal(map[string]any{"id":"obs-dead-1","title":"t","content":"c","session_id":"missing","project":"proj-dead"})
 m:=SyncMutation{Project:"proj-dead",Entity:"observation",EntityKey:"obs-dead-1",Payload:pl}
 _=s.ApplyPulledMutation(m)
 _,_=s.db.Exec(`UPDATE sync_apply_deferred SET attempts=5 WHERE entity_key='obs-dead-1'`)
 if err:=s.ApplyPulledMutation(m);!errors.Is(err,ErrFKMissing){t.Fatalf("expected fk %v",err)}
 var att int
 var sid string
 _=s.db.QueryRow(`SELECT sync_id,attempts FROM sync_apply_deferred WHERE entity_key=?`,"obs-dead-1").Scan(&sid,&att)
 if att!=6{t.Fatalf("att=%d want 6",att)}
 want:=deadLetterID("proj-dead","obs-dead-1",pl)
 if sid!=want{t.Fatalf("dead %q want %q",sid,want)}
 if relationApplyFailureSyncID("proj-dead","obs-dead-1",pl)==want{t.Fatalf("collision")}
}
func TestDeferredReplay(t *testing.T){
 s:=openTestStore(t)
 _,_=s.db.Exec(`INSERT INTO sessions (id,start_time,project,leaf_id) VALUES ('sess-replay','2000-01-01T00:00:00Z','proj-replay','sess-replay')`)
 plMiss,_:=json.Marshal(map[string]any{"id":"obs-replay-miss","title":"miss","content":"c","session_id":"missing2","project":"proj-replay"})
 mMiss:=SyncMutation{Project:"proj-replay",Entity:"observation",EntityKey:"obs-replay-miss",Payload:plMiss}
 _=s.ApplyPulledMutation(mMiss)
 plOK,_:=json.Marshal(map[string]any{"id":"obs-replay-ok","title":"ok","content":"c","session_id":"sess-replay","project":"proj-replay"})
 sidOK:=pulledSessionDeadLetterSyncID("proj-replay","obs-replay-ok",plOK)
 _,_=s.db.Exec(`INSERT INTO sync_apply_deferred (sync_id,project,entity,entity_key,payload,attempts,created_at) VALUES (?,?,?,?,?,?,?)`,sidOK,"proj-replay","observation","obs-replay-ok",string(plOK),1,"now")
 if err:=s.ReplayDeferredForScope("proj-replay");err!=nil{t.Fatalf("Replay %v",err)}
 var cnt int
 _=s.db.QueryRow(`SELECT COUNT(*) FROM sync_apply_deferred WHERE entity_key='obs-replay-ok'`).Scan(&cnt)
 if cnt!=0{t.Fatalf("cleared ok cnt=%d",cnt)}
 _=s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id='obs-replay-ok'`).Scan(&cnt)
 if cnt!=1{t.Fatalf("obs not applied cnt=%d",cnt)}
 var att int
 _=s.db.QueryRow(`SELECT attempts FROM sync_apply_deferred WHERE entity_key='obs-replay-miss'`).Scan(&att)
 if att!=2{t.Fatalf("miss att=%d want 2",att)}
 plOther,_:=json.Marshal(map[string]any{"id":"obs-other","title":"o","content":"c","session_id":"missing","project":"proj-other"})
 mOther:=SyncMutation{Project:"proj-other",Entity:"observation",EntityKey:"obs-other",Payload:plOther}
 _=s.ApplyPulledMutation(mOther)
 _=s.ReplayDeferredForScope("proj-replay")
 var other int
 _=s.db.QueryRow(`SELECT attempts FROM sync_apply_deferred WHERE entity_key='obs-other'`).Scan(&other)
 if other!=1{t.Fatalf("leak other %d",other)}
}
func TestDeferredCoexist(t *testing.T){
 s:=openTestStore(t)
 _,_=s.db.Exec(`INSERT INTO sync_chunks (target_key,chunk_id) VALUES ('engram:','abc123')`)
 _,_=s.db.Exec(`INSERT INTO observations (id,title,project,created_at,updated_at) VALUES ('obs-coexist','t','proj-coexist','now','now')`)
 tx,_:=s.db.Begin()
 _,err:=enqueueSyncMutationTx(tx,"proj-coexist","observation","obs-coexist","upsert",[]byte(`{}`))
 if err!=nil{t.Fatalf("enqueue %v",err)}
 _=tx.Commit()
 var c1,c2 int
 _=s.db.QueryRow(`SELECT COUNT(*) FROM sync_chunks WHERE chunk_id='abc123'`).Scan(&c1)
 _=s.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE project='proj-coexist'`).Scan(&c2)
 if c1!=1||c2!=1{t.Fatalf("coexist %d %d",c1,c2)}
}
func TestDeadLetterHash(t *testing.T){TestDeferredDeadLetterHash(t)}
func TestReplayDeferredForScope(t *testing.T){TestDeferredReplay(t)}
