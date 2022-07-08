//#define EF_NONE     			  0	// * EFFECT Type이 0 이면 이펙트가 없다.  ITEM에서 Score[0]의 값을 더할수 없다.
// Score 시작
#define EF_LEVEL     			  1	// 레벨 * Score[1] - Score[15] Bonus Score값을 그대로 더해 버린다.
#define EF_DAMAGE                 2 // 증뎀
#define EF_AC       			  3	// 방어
#define EF_HP        			  4	// 피증  HP,MP,공격속도,이동속도,각종 MAX등등
#define EF_MP       			  5	// 마증
#define EF_EXP      			  6	// 경험
#define EF_STR      		      7	// 힘
#define EF_INT      		      8	// 정신 
#define EF_DEX       		      9	// 민첩
#define EF_CON      		     10	// 체격
#define EF_SPECIAL1    		     11	// 무마
#define EF_SPECIAL2    			 12	// 특1
#define EF_SPECIAL3    			 13	// 특2
#define EF_SPECIAL4    			 14	// 특3
// Score 끝.
#define EF_SCORE14     			 15	// 
#define EF_SCORE15     			 16	// 
////////////////      REQUIREMENT        //////////////////////////////////////////
#define EF_POS                   17  // 위치 입을수 있는 장소 bit값         - 디폴트 NOWHERE (0비트 가 EQUIP 0)
#define EF_CLASS                 18  // 직업 입을수 있는 (성별)클래스 bit값 - 디폴트 YES
#define EF_R1SIDC                19  // 한손 1핸드 또는 갑옷의 입을수 있는 STR INT DEC CON         - 디폴트 0
#define EF_R2SIDC                20  // 두손 2핸드          의 입을수 있는 STR INT DEC CON         - 디폴트 불가
////////////////   BONUS  ////////////////////////////////
#define EF_WTYPE         	     21	 // 무류 무기분류 (애니메이션) 
                                     //  1:소검(클럽)
                                     //  2:중검(메이스)    
                                     //  3:대검               

                                     // 11:소도끼         
                                     // 12:중도끼             
                                     // 13:대도끼             

                                     // 21:소창               
                                     // 22:중창               
                                     // 23:대창               

                                     // 31:소지팡이            
                                     // 32:중지팡이           
                                     // 33:대지팡이           
 
                                     // 41:소클로
                                     
                                     // 51:소방패
                                    
                                     // 61:소클럽
                                     // 62:중클럽
                                     // 63:대클럽 

                                     // 101:활
                                     // 102:다트 
                                     // 103:토마호크
                                     // 104:재블린   
////////////////////////////////////////////////////////////////////////////
#define EF_REQ_STR               22  // 힘제 
#define EF_REQ_INT               23  // 정제
#define EF_REQ_DEX               24  // 민제
#define EF_REQ_CON               25  // 콘제
/////////////////////////////////////////////////////////////////////////
// * 주 요구치이나, 아이템은 기본 SIDC에 추가되는 값이다.
/////////////////////////////////////////////////////////////////////////


#define EF_ATTSPEED  	       	 26	 // 공속 공격속도
#define EF_RANGE                 27  // 사정 사정거리. 전체중 최대값이 적용됨.
////#define EF_PRICE				 28	 // 가격
#define EF_RUNSPEED              29  // 런닝 달리기속도.
#define EF_SPELL    	    	 30	 // 스펠 Item을 먹거나 입었을때 걸리는 마법
#define EF_DURATION              31  // 지속 Item 파라메타1 - 먹을때 효력이 발생하는 시간, 입는아이템은 항상 
#define EF_PARM2     			 32	 // 파람 Item 파라메타2
#define EF_GRID                  33  // 크기 Carry를 차지하는 아이템그리드
#define EF_GROUND                34  // 땅칸 성문등에서 땅을 차지하는 크기
#define EF_CLAN                  35  // 종족
#define EF_HWORDCOIN             36  // 대돈 
#define EF_LWORDCOIN             37  // 소돈 
#define EF_VOLATILE              38  // 소모 먹으면 바로 사라지는 아이템.  0:일반장구  1:물약계통  2:돈계통   3:열쇠 4:축1  5:축2  6:축3
                                     //                               6:스펠+15   7:레벨+1
#define EF_KEYID                 39  // 키번
#define EF_PARRY                 40  // 회피 회피율 보나스
#define EF_HITRATE               41  // 밍중 명중률 보나스
#define EF_CRITICAL              42  // 크리 크리티컬
#define EF_SANC                  43  // 제련 Sanctuary 주로 보나스 포인트에 편성되며 
#define EF_SAVEMANA              44  // 마절 1 -> 99%
#define EF_HPADD                 45  // 피뻥 1 -> 101%   MAX_MP 
#define EF_MPADD                 46  // 마뻥 1 -> 101%   MAX_MP
#define EF_REGENHP               47  // 피회
#define EF_REGENMP               48  // 마회
#define EF_RESIST1               49  // 불저
#define EF_RESIST2               50  // 얼저
#define EF_RESIST3               51  // 신저
#define EF_RESIST4               52  // 번저
#define EF_ACADD                 53  // 방증
#define EF_RESISTALL             54  // 올저
#define EF_BONUS                 55  // 보나
#define EF_HWORDGUILD            56  // 길드
#define EF_LWORDGUILD            57  // 길드
#define EF_QUEST                 58  // 임무 반드시 1부터 시작, 대부분 Volatile과 함게 세팅될 것 임.
#define EF_UNIQUE                59  // 고유
#define EF_MAGIC                 60  // 마공 1 -> 1% 증폭 
#define EF_AMOUNT                61  // 갯수
#define EF_HWORDINDEX            62  // 번호
#define EF_LWORDINDEX            63  // 번호
#define EF_INIT1                 64  // 초기
#define EF_INIT2                 65  // 초기
#define EF_INIT3                 66  // 초기
#define EF_DAMAGEADD             67  // 댐뽀
#define EF_MAGICADD              68  // 마뽀
                                     // 다음 2들은 별로도 쿼리를 던지지 않고 베이스를 쿼리를 던지면 유닉 옵에서 자동 체킹하는 부분.   
                                     // 클라이언트 표기시는 이것을 먼저 던저서 이것이 나오면 베이스는 던지지 않는다.
#define EF_HPADD2                69  // 피2   피뻥과 같으나 기본 옵에 포함되어 표시된다.
#define EF_MPADD2                70  // 피2   피뻥과 같으나 기본 옵에 포함되어 표시된다.
#define EF_CRITICAL2             71  // 크리 크리티컬
#define EF_ACADD2                72  // 방증
#define EF_DAMAGE2               73  // 
#define EF_SPECIALALL            74  // 마스터리 무마제외 증가 

#define	EF_CURKILL				 75  // not used
#define EF_LTOTKILL				 76  // not used
#define EF_HTOTKILL				 77  // not used
#define EF_INCUBATE				 78  // 부화임계치


#define EF_MOUNTLIFE			 79  // 말 총 생명 
#define EF_MOUNTHP				 80  // 말 현 피
#define EF_MOUNTSANC			 81  // 말 성장도
#define EF_MOUNTFEED			 82  // 말 배부름 정도	
#define EF_MOUNTKILL			 83  // 말이 죽인 몬스터 갯수?

#define EF_INCUDELAY             84
#define EF_SUBGUILD				 85	// SUB길드, 0이면 최대길드 1,2,3까지 있다

#define EF_MOBFACE				 86

#define EF_GRADE0				 100 // 0급
#define EF_GRADE1				 101 // 1급
#define EF_GRADE2				 102 // 2급
#define EF_GRADE3				 103 // 3급
#define EF_GRADE4				 104 // 4급
#define EF_GRADE5				 105 // 5급

#define EF_MOBTYPE				 112
#define EF_ITEMTYPE				 116

//00488D32 eip
//009ffbcc 
//44d65000767